package handlers

import (
	"context"
	"errors"
	"io"
	"os"
	"syscall"
	"testing"
)

func TestIsExpectedStreamTermination(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "context canceled", err: context.Canceled, want: true},
		{name: "io eof", err: io.EOF, want: true},
		{name: "wrapped context canceled", err: errors.Join(errors.New("read failed"), context.Canceled), want: true},
		{name: "broken pipe syscall", err: &os.SyscallError{Syscall: "write", Err: syscall.EPIPE}, want: true},
		{name: "connection reset syscall", err: &os.SyscallError{Syscall: "read", Err: syscall.ECONNRESET}, want: true},
		{name: "string match", err: errors.New("stream aborted: context canceled"), want: true},
		{name: "real error", err: errors.New("tls handshake timeout"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isExpectedStreamTermination(tt.err); got != tt.want {
				t.Fatalf("isExpectedStreamTermination(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsExpectedRequestReadError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "unexpected eof sentinel", err: io.ErrUnexpectedEOF, want: true},
		{name: "context canceled", err: context.Canceled, want: true},
		{name: "connection reset syscall", err: &os.SyscallError{Syscall: "read", Err: syscall.ECONNRESET}, want: true},
		{name: "wrapped unexpected eof", err: errors.Join(errors.New("body read failed"), io.ErrUnexpectedEOF), want: true},
		{name: "non client error", err: errors.New("tls handshake timeout"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isExpectedRequestReadError(tt.err); got != tt.want {
				t.Fatalf("isExpectedRequestReadError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsExpectedRequestParseError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "truncated json", err: errors.New("unexpected end of JSON input"), want: true},
		{name: "unexpected eof", err: io.ErrUnexpectedEOF, want: true},
		{name: "syntax error", err: errors.New("invalid character '}' looking for beginning of object key string"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isExpectedRequestParseError(tt.err); got != tt.want {
				t.Fatalf("isExpectedRequestParseError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
