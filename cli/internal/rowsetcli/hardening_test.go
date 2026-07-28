package rowsetcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

type failingWriter struct {
	err error
}

type failingReadCloser struct {
	err error
}

func (reader failingReadCloser) Read(_ []byte) (int, error) {
	return 0, reader.err
}

func (failingReadCloser) Close() error {
	return nil
}

func (writer failingWriter) Write(_ []byte) (int, error) {
	return 0, writer.err
}

type signalingWriter struct {
	once   sync.Once
	wrote  chan struct{}
	mu     sync.Mutex
	output bytes.Buffer
}

func newSignalingWriter() *signalingWriter {
	return &signalingWriter{wrote: make(chan struct{})}
}

func (writer *signalingWriter) Write(data []byte) (int, error) {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	writer.once.Do(func() {
		close(writer.wrote)
	})
	return writer.output.Write(data)
}

func (writer *signalingWriter) String() string {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.output.String()
}

func TestRunRejectsUnexpectedOperands(t *testing.T) {
	tests := [][]string{
		{"capabilities", "unexpected"},
		{"healthcheck", "unexpected"},
		{"user", "info", "unexpected"},
		{"project", "list", "unexpected"},
		{"feedback", "submit", "--feedback", "hello", "unexpected"},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			err := Run(context.Background(), IO{
				Stdin:  strings.NewReader(""),
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
			}, args)
			if err == nil {
				t.Fatal("expected unexpected operand to fail")
			}
			if got := ExitCode(err); got != ExitUsage {
				t.Fatalf("exit code mismatch: got %d want %d (error: %v)", got, ExitUsage, err)
			}
		})
	}
}

func TestRunValidatesPaginationAndPreviewRanges(t *testing.T) {
	tests := [][]string{
		{"project", "list", "--limit", "0"},
		{"dataset", "list", "--limit", "101"},
		{"row", "list", "dataset-key", "--offset", "-1"},
		{"row", "search", "query", "--limit", "51"},
		{"preview", "update", "dataset-key", "--page-size", "101"},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			err := Run(context.Background(), IO{
				Stdin:  strings.NewReader(""),
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
			}, args)
			if err == nil {
				t.Fatal("expected invalid range to fail")
			}
			if got := ExitCode(err); got != ExitUsage {
				t.Fatalf("exit code mismatch: got %d want %d (error: %v)", got, ExitUsage, err)
			}
		})
	}
}

func TestDatasetRowsMustContainObjects(t *testing.T) {
	err := Run(context.Background(), IO{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}, []string{"dataset", "create", "--name", "Invalid", "--rows", `[{"ok":true}, 7]`})
	if err == nil {
		t.Fatal("expected non-object row to fail")
	}
	if got := ExitCode(err); got != ExitUsage {
		t.Fatalf("exit code mismatch: got %d want %d", got, ExitUsage)
	}
}

func TestPreviewPasswordUsesStdinOrEnvironment(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		env   map[string]string
		want  string
	}{
		{
			name:  "stdin",
			args:  []string{"preview", "update", "dataset-key", "--password-stdin"},
			stdin: "stdin-secret\n",
			want:  "stdin-secret",
		},
		{
			name: "environment",
			args: []string{
				"preview", "update", "dataset-key", "--password-env", "ROWSET_PREVIEW_PASSWORD",
			},
			env:  map[string]string{"ROWSET_PREVIEW_PASSWORD": "env-secret"},
			want: "env-secret",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for name, value := range test.env {
				t.Setenv(name, value)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				body := map[string]any{}
				if err := decodeJSONBody(request.Body, &body); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				if got := body["public_password"]; got != test.want {
					t.Fatalf("password mismatch: got %q want %q", got, test.want)
				}
				_, _ = w.Write([]byte(`{"status":"ok"}`))
			}))
			t.Cleanup(server.Close)
			t.Setenv("ROWSET_API_BASE", server.URL+"/api/")
			t.Setenv("ROWSET_API_KEY", "test-key")

			err := Run(context.Background(), IO{
				Stdin:  strings.NewReader(test.stdin),
				Stdout: &bytes.Buffer{},
				Stderr: &bytes.Buffer{},
			}, test.args)
			if err != nil {
				t.Fatalf("Run returned error: %v", err)
			}
		})
	}
}

func TestPreviewPasswordArgumentIsRejected(t *testing.T) {
	err := Run(context.Background(), IO{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}, []string{"preview", "update", "dataset-key", "--password", "visible-secret"})
	if err == nil {
		t.Fatal("expected --password to be rejected")
	}
	if got := ExitCode(err); got != ExitUsage {
		t.Fatalf("exit code mismatch: got %d want %d", got, ExitUsage)
	}
}

func TestHTTPClientTimeoutIsPreservedAndClassified(t *testing.T) {
	t.Setenv("ROWSET_API_BASE", "https://rowset.example/api/")
	t.Setenv("ROWSET_API_KEY", "test-key")
	client := &http.Client{
		Timeout: 20 * time.Millisecond,
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		}),
	}

	err := Run(context.Background(), IO{
		Stdin:      strings.NewReader(""),
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		HTTPClient: client,
	}, []string{"user", "info"})
	if err == nil {
		t.Fatal("expected timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout identity was not preserved: %v", err)
	}
	if got := ExitCode(err); got != ExitNetwork {
		t.Fatalf("exit code mismatch: got %d want %d", got, ExitNetwork)
	}
}

func TestDefaultHTTPClientHasFiniteBudgets(t *testing.T) {
	client := newHTTPClient()
	if client.Timeout <= 0 {
		t.Fatal("default HTTP client must have a total timeout")
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("unexpected transport type: %T", client.Transport)
	}
	if transport.ResponseHeaderTimeout <= 0 || transport.TLSHandshakeTimeout <= 0 {
		t.Fatalf(
			"transport timeouts must be finite: header=%s TLS=%s",
			transport.ResponseHeaderTimeout,
			transport.TLSHandshakeTimeout,
		)
	}
}

func TestExitCodeCancellation(t *testing.T) {
	for _, err := range []error{
		context.Canceled,
		fmt.Errorf("wrapped cancellation: %w", context.Canceled),
	} {
		if got := ExitCode(err); got != ExitCanceled {
			t.Fatalf("exit code mismatch: got %d want %d", got, ExitCanceled)
		}
	}
}

func TestMissingAPIKeyIsAnAuthenticationFailure(t *testing.T) {
	t.Setenv("ROWSET_API_BASE", "https://rowset.example/api/")
	t.Setenv("ROWSET_API_KEY", "")

	err := Run(context.Background(), IO{
		Stdin:  strings.NewReader(""),
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
	}, []string{"user", "info"})
	if err == nil {
		t.Fatal("expected missing API key to fail")
	}
	if got := ExitCode(err); got != ExitAuth {
		t.Fatalf("exit code mismatch: got %d want %d", got, ExitAuth)
	}
}

func TestUnexpectedRedirectStatusIsAnError(t *testing.T) {
	t.Setenv("ROWSET_API_BASE", "https://rowset.example/api/")
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusMultipleChoices,
				Status:     "300 Multiple Choices",
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("choose another endpoint")),
				Request:    request,
			}, nil
		}),
	}

	err := Run(context.Background(), IO{
		Stdin:      strings.NewReader(""),
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		HTTPClient: client,
	}, []string{"healthcheck"})
	if err == nil {
		t.Fatal("expected 300 response to fail")
	}
	var requestError *RequestError
	if !errors.As(err, &requestError) {
		t.Fatalf("expected RequestError, got %T: %v", err, err)
	}
	if requestError.StatusCode != http.StatusMultipleChoices {
		t.Fatalf("status mismatch: got %d", requestError.StatusCode)
	}
}

func TestDefaultClientDoesNotFollowCrossOriginRedirects(t *testing.T) {
	targetReached := make(chan string, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		targetReached <- request.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(target.Close)
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Redirect(w, request, target.URL, http.StatusFound)
	}))
	t.Cleanup(source.Close)
	t.Setenv("ROWSET_API_BASE", source.URL+"/")
	t.Setenv("ROWSET_API_KEY", "test-key")

	err := Run(context.Background(), IO{
		Stdin:      strings.NewReader(""),
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		HTTPClient: newHTTPClient(),
	}, []string{"user", "info"})
	var requestError *RequestError
	if !errors.As(err, &requestError) || requestError.StatusCode != http.StatusFound {
		t.Fatalf("expected redirect response error, got %v", err)
	}
	select {
	case authorization := <-targetReached:
		t.Fatalf("redirect target received request with authorization %q", authorization)
	default:
	}
}

func TestErrorResponseReadFailureIsPreserved(t *testing.T) {
	sentinel := errors.New("response read failed")
	t.Setenv("ROWSET_API_BASE", "https://rowset.example/api/")
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Status:     "500 Internal Server Error",
				Header:     make(http.Header),
				Body:       failingReadCloser{err: sentinel},
				Request:    request,
			}, nil
		}),
	}

	err := Run(context.Background(), IO{
		Stdin:      strings.NewReader(""),
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		HTTPClient: client,
	}, []string{"healthcheck"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("response read failure was not preserved: %v", err)
	}
}

func TestJSONResponseBodyIsBounded(t *testing.T) {
	t.Setenv("ROWSET_API_BASE", "https://rowset.example/api/")
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body: io.NopCloser(io.LimitReader(
					zeroReader{},
					maxJSONResponseBytes+1,
				)),
				Request: request,
			}, nil
		}),
	}

	err := Run(context.Background(), IO{
		Stdin:      strings.NewReader(""),
		Stdout:     &bytes.Buffer{},
		Stderr:     &bytes.Buffer{},
		HTTPClient: client,
	}, []string{"healthcheck"})
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected bounded response error, got %v", err)
	}
}

type zeroReader struct{}

func (zeroReader) Read(data []byte) (int, error) {
	for index := range data {
		data[index] = 'x'
	}
	return len(data), nil
}

func TestRawResponseStreamsBeforeEOF(t *testing.T) {
	t.Setenv("ROWSET_API_BASE", "https://rowset.example/api/")
	t.Setenv("ROWSET_API_KEY", "test-key")
	reader, writer := io.Pipe()
	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Header:     make(http.Header),
				Body:       reader,
				Request:    request,
			}, nil
		}),
	}
	stdout := newSignalingWriter()
	runDone := make(chan error, 1)
	go func() {
		runDone <- Run(context.Background(), IO{
			Stdin:      strings.NewReader(""),
			Stdout:     stdout,
			Stderr:     &bytes.Buffer{},
			HTTPClient: client,
		}, []string{"export", "dataset-key", "csv"})
	}()

	if _, err := writer.Write([]byte("first")); err != nil {
		t.Fatalf("write first chunk: %v", err)
	}
	select {
	case <-stdout.wrote:
	case <-time.After(250 * time.Millisecond):
		_ = writer.Close()
		<-runDone
		t.Fatal("raw response was buffered instead of streamed")
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close response: %v", err)
	}
	if err := <-runDone; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); got != "first" {
		t.Fatalf("output mismatch: got %q", got)
	}
}

func TestOutputFileReplacementIsPrivate(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "export.csv")
	if err := os.WriteFile(outputPath, []byte("old"), 0o644); err != nil {
		t.Fatalf("write existing output: %v", err)
	}
	if err := os.Chmod(outputPath, 0o644); err != nil {
		t.Fatalf("chmod existing output: %v", err)
	}

	runAgainstServer(t, []string{
		"export", "dataset-key", "csv", "--output", outputPath,
	}, func(w http.ResponseWriter, request *http.Request) {
		_, _ = w.Write([]byte("new"))
	})

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("output mode mismatch: got %o want 600", got)
	}
}

func TestOutputFileWriteFailurePreservesExistingFile(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "export.csv")
	if err := os.WriteFile(outputPath, []byte("old"), 0o600); err != nil {
		t.Fatalf("write existing output: %v", err)
	}
	sentinel := errors.New("stream failed")
	source := io.MultiReader(strings.NewReader("partial"), failingReadCloser{err: sentinel})

	err := writeAtomicFile(outputPath, source)
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected stream failure, got %v", err)
	}
	data, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("read existing output: %v", readErr)
	}
	if got := string(data); got != "old" {
		t.Fatalf("existing output changed: got %q", got)
	}
}

func TestHelpReturnsWriteFailures(t *testing.T) {
	sentinel := errors.New("write failed")
	err := Run(context.Background(), IO{
		Stdin:  strings.NewReader(""),
		Stdout: failingWriter{err: sentinel},
		Stderr: &bytes.Buffer{},
	}, []string{"--help"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected write failure, got %v", err)
	}
}

func TestBrokenPipeIsNormalTermination(t *testing.T) {
	err := Run(context.Background(), IO{
		Stdin:  strings.NewReader(""),
		Stdout: failingWriter{err: syscall.EPIPE},
		Stderr: &bytes.Buffer{},
	}, []string{"--version"})
	if err == nil {
		t.Fatal("expected writer to return EPIPE")
	}
	if got := ExitCode(err); got != ExitSuccess {
		t.Fatalf("exit code mismatch: got %d want %d", got, ExitSuccess)
	}
}

func decodeJSONBody(reader io.Reader, target any) error {
	defer func() {
		if closer, ok := reader.(io.Closer); ok {
			_ = closer.Close()
		}
	}()
	return json.NewDecoder(reader).Decode(target)
}
