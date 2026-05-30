package handlers

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
)

func isExpectedStreamTermination(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return true
	}

	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		if errors.Is(syscallErr.Err, syscall.EPIPE) || errors.Is(syscallErr.Err, syscall.ECONNRESET) {
			return true
		}
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context canceled") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "client disconnected") ||
		strings.Contains(msg, "unexpected eof")
}

func streamTerminationReason(err error) string {
	if err == nil {
		return ""
	}

	switch {
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, net.ErrClosed):
		return "net_closed"
	case errors.Is(err, io.EOF):
		return "eof"
	}

	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		switch {
		case errors.Is(syscallErr.Err, syscall.EPIPE):
			return "broken_pipe"
		case errors.Is(syscallErr.Err, syscall.ECONNRESET):
			return "connection_reset"
		}
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "context canceled"):
		return "context_canceled"
	case strings.Contains(msg, "broken pipe"):
		return "broken_pipe"
	case strings.Contains(msg, "connection reset by peer"):
		return "connection_reset"
	case strings.Contains(msg, "client disconnected"):
		return "client_disconnected"
	case strings.Contains(msg, "unexpected eof"):
		return "unexpected_eof"
	default:
		return "read_error"
	}
}

func isExpectedRequestReadError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed) {
		return true
	}

	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		if errors.Is(syscallErr.Err, syscall.EPIPE) || errors.Is(syscallErr.Err, syscall.ECONNRESET) {
			return true
		}
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context canceled") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "client disconnected") ||
		strings.Contains(msg, "unexpected eof")
}

func requestReadErrorReason(err error) string {
	if err == nil {
		return ""
	}

	switch {
	case errors.Is(err, context.Canceled):
		return "context_canceled"
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "unexpected_eof"
	case errors.Is(err, io.EOF):
		return "eof"
	case errors.Is(err, net.ErrClosed):
		return "net_closed"
	}

	var syscallErr *os.SyscallError
	if errors.As(err, &syscallErr) {
		switch {
		case errors.Is(syscallErr.Err, syscall.EPIPE):
			return "broken_pipe"
		case errors.Is(syscallErr.Err, syscall.ECONNRESET):
			return "connection_reset"
		}
	}

	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "context canceled"):
		return "context_canceled"
	case strings.Contains(msg, "broken pipe"):
		return "broken_pipe"
	case strings.Contains(msg, "connection reset by peer"):
		return "connection_reset"
	case strings.Contains(msg, "client disconnected"):
		return "client_disconnected"
	case strings.Contains(msg, "unexpected eof"):
		return "unexpected_eof"
	default:
		return "read_error"
	}
}

func isExpectedRequestParseError(err error) bool {
	if err == nil {
		return false
	}

	if isExpectedRequestReadError(err) {
		return true
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unexpected end of json input")
}

func requestParseErrorReason(err error) string {
	if err == nil {
		return ""
	}

	if isExpectedRequestReadError(err) {
		return requestReadErrorReason(err)
	}

	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "unexpected end of json input") {
		return "truncated_json"
	}

	return "parse_error"
}

func logRequestReadFailure(ctx context.Context, r *http.Request, endpoint string, err error) {
	fields := log.Fields{
		"endpoint":       endpoint,
		"method":         r.Method,
		"path":           r.URL.Path,
		"remote_addr":    r.RemoteAddr,
		"content_length": r.ContentLength,
		"reason":         requestReadErrorReason(err),
	}
	entry := loggerWithTrace(ctx).WithError(err).WithFields(fields)
	if isExpectedRequestReadError(err) {
		entry.Warn("Request body terminated before full payload was received")
		return
	}
	entry.Error("Request body read failed")
}

func logRequestParseFailure(ctx context.Context, r *http.Request, endpoint string, err error) {
	fields := log.Fields{
		"endpoint":       endpoint,
		"method":         r.Method,
		"path":           r.URL.Path,
		"remote_addr":    r.RemoteAddr,
		"content_length": r.ContentLength,
		"reason":         requestParseErrorReason(err),
	}
	entry := loggerWithTrace(ctx).WithError(err).WithFields(fields)
	if isExpectedRequestParseError(err) {
		entry.Warn("Request payload ended before a complete JSON document was received")
		return
	}
	entry.Error("Request JSON parse failed")
}
