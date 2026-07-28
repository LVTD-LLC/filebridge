package rowsetcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultRequestTimeout = 5 * time.Minute
	maxJSONResponseBytes  = 16 * 1024 * 1024
	maxErrorResponseBytes = 1 * 1024 * 1024
)

var (
	defaultClientOnce sync.Once
	defaultClient     *http.Client
)

type RequestError struct {
	StatusCode int
	Code       string
	Message    string
	UpgradeURL string
}

func (err *RequestError) Error() string {
	details := make([]string, 0, 3)
	if err.Code != "" {
		details = append(details, err.Code)
	}
	if err.Message != "" {
		details = append(details, err.Message)
	}
	if err.UpgradeURL != "" {
		details = append(details, "Upgrade: "+err.UpgradeURL)
	}
	if len(details) > 0 {
		return "Rowset couldn't complete the request — " + strings.Join(details, " — ")
	}
	switch err.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return "Rowset authentication failed. Check the configured API key and its access level."
	case http.StatusTooManyRequests:
		return "Rowset is rate limiting requests. Wait and try again."
	default:
		return fmt.Sprintf(
			"Rowset couldn't complete the request (HTTP %d). Check the command and try again.",
			err.StatusCode,
		)
	}
}

func newHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	transport.TLSHandshakeTimeout = 10 * time.Second
	transport.ResponseHeaderTimeout = 30 * time.Second

	return &http.Client{
		Transport: transport,
		Timeout:   defaultRequestTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) == 0 {
				return nil
			}
			origin := via[0].URL
			if !strings.EqualFold(request.URL.Scheme, origin.Scheme) ||
				!strings.EqualFold(request.URL.Host, origin.Host) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

func requestClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	defaultClientOnce.Do(func() {
		defaultClient = newHTTPClient()
	})
	return defaultClient
}

func doRequest(
	ctx context.Context,
	streams IO,
	cfg config,
	method string,
	path string,
	query url.Values,
	opts requestOptions,
) (returnErr error) {
	endpoint, err := buildEndpoint(cfg.apiBase, path, query, !opts.auth)
	if err != nil {
		return err
	}

	var body io.Reader
	if opts.body != nil {
		data, err := json.Marshal(opts.body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		body = bytes.NewReader(data)
	} else if opts.bodyBytes != nil {
		body = bytes.NewReader(opts.bodyBytes)
	}

	request, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return wrapUsageError(err)
	}
	request.Header.Set("User-Agent", "rowset/"+Version)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if opts.auth {
		apiKey := strings.TrimSpace(os.Getenv(cfg.apiKeyEnv))
		if apiKey == "" {
			return authErrorf(
				"%s is required for authenticated Rowset requests",
				cfg.apiKeyEnv,
			)
		}
		request.Header.Set("Authorization", "Bearer "+apiKey)
	}

	response, err := requestClient(streams.HTTPClient).Do(request)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("request canceled: %w", ctxErr)
		}
		return fmt.Errorf("send Rowset request: %w", err)
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close response body: %w", closeErr))
		}
	}()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		responseBody, readErr := readBounded(response.Body, maxErrorResponseBytes)
		if readErr != nil {
			return fmt.Errorf("read error response: %w", readErr)
		}
		return safeRequestError(response.StatusCode, responseBody)
	}
	if opts.outputPath != "" {
		return writeAtomicFile(opts.outputPath, response.Body)
	}
	if opts.rawOutput {
		_, err = io.Copy(streams.Stdout, response.Body)
		return err
	}

	responseBody, err := readBounded(response.Body, maxJSONResponseBytes)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if len(responseBody) == 0 {
		return nil
	}
	formatted := formatJSON(responseBody, cfg.compact)
	_, err = streams.Stdout.Write(formatted)
	return err
}

func safeRequestError(statusCode int, responseBody []byte) error {
	payload := apiErrorResponse{}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return &RequestError{StatusCode: statusCode}
	}
	return &RequestError{
		StatusCode: statusCode,
		Code:       strings.TrimSpace(payload.Code),
		Message:    firstNonBlank(payload.Message, payload.Detail),
		UpgradeURL: strings.TrimSpace(payload.UpgradeURL),
	}
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	limited := &io.LimitedReader{R: reader, N: limit + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response body exceeds %d bytes", limit)
	}
	return data, nil
}

func writeAtomicFile(path string, source io.Reader) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure temporary output: %w", err)
	}
	if _, err := io.Copy(temporary, source); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write output: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync output: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close output: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace output: %w", err)
	}
	return nil
}
