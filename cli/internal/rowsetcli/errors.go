package rowsetcli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"syscall"
)

const (
	ExitSuccess  = 0
	ExitFailure  = 1
	ExitUsage    = 2
	ExitAuth     = 3
	ExitNetwork  = 4
	ExitRemote   = 5
	ExitCanceled = 130
)

type UsageError struct {
	Err error
}

type AuthError struct {
	Err error
}

func (err *AuthError) Error() string {
	return err.Err.Error()
}

func (err *AuthError) Unwrap() error {
	return err.Err
}

func (err *UsageError) Error() string {
	return err.Err.Error()
}

func (err *UsageError) Unwrap() error {
	return err.Err
}

func usageError(message string) error {
	return &UsageError{Err: errors.New(message)}
}

func usageErrorf(format string, args ...any) error {
	return &UsageError{Err: fmt.Errorf(format, args...)}
}

func wrapUsageError(err error) error {
	if err == nil {
		return nil
	}
	var existing *UsageError
	if errors.As(err, &existing) {
		return err
	}
	return &UsageError{Err: err}
}

func authErrorf(format string, args ...any) error {
	return &AuthError{Err: fmt.Errorf(format, args...)}
}

func ExitCode(err error) int {
	if err == nil || errors.Is(err, syscall.EPIPE) {
		return ExitSuccess
	}
	if errors.Is(err, context.Canceled) {
		return ExitCanceled
	}
	var usage *UsageError
	if errors.As(err, &usage) {
		return ExitUsage
	}
	var auth *AuthError
	if errors.As(err, &auth) {
		return ExitAuth
	}
	var request *RequestError
	if errors.As(err, &request) {
		if request.StatusCode == 401 || request.StatusCode == 403 {
			return ExitAuth
		}
		return ExitRemote
	}
	var network net.Error
	if errors.As(err, &network) || errors.Is(err, context.DeadlineExceeded) {
		return ExitNetwork
	}
	return ExitFailure
}
