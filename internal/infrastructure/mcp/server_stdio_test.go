package mcp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestStdioTransport_RunRejectsUnconfiguredServer(t *testing.T) {
	var nilTransport *StdioTransport
	if err := nilTransport.Run(context.Background()); err == nil || err.Error() != "stdio transport server is not configured" {
		t.Fatalf("expected nil transport server-configured error, got %v", err)
	}

	transport := NewStdioTransport(nil, bytes.NewBuffer(nil), bytes.NewBuffer(nil))
	if err := transport.Run(context.Background()); err == nil || err.Error() != "stdio transport server is not configured" {
		t.Fatalf("expected nil server configured error, got %v", err)
	}
}

func TestStdioTransport_RunRejectsNilContext(t *testing.T) {
	server := NewServer(Options{})
	transport := NewStdioTransport(server, bytes.NewBuffer(nil), bytes.NewBuffer(nil))

	if err := transport.Run(nil); err == nil || err.Error() != "context is required" {
		t.Fatalf("expected context-required error, got %v", err)
	}
}

func TestStdioTransport_RunRejectsNilReaderOrWriter(t *testing.T) {
	server := NewServer(Options{})

	if err := NewStdioTransport(server, nil, bytes.NewBuffer(nil)).Run(context.Background()); err == nil || err.Error() != "reader is required" {
		t.Fatalf("expected reader-required error, got %v", err)
	}
	if err := NewStdioTransport(server, bytes.NewBuffer(nil), nil).Run(context.Background()); err == nil || err.Error() != "writer is required" {
		t.Fatalf("expected writer-required error, got %v", err)
	}
	if err := NewStdioTransport(server, nil, nil).Run(context.Background()); err == nil || err.Error() != "reader is required" {
		t.Fatalf("expected reader-required precedence when both are nil, got %v", err)
	}
}

func TestStdioTransport_RunValidationPrecedence(t *testing.T) {
	server := NewServer(Options{})

	if err := NewStdioTransport(nil, nil, nil).Run(nil); err == nil || err.Error() != "stdio transport server is not configured" {
		t.Fatalf("expected server-configured precedence error, got %v", err)
	}
	if err := NewStdioTransport(server, nil, nil).Run(nil); err == nil || err.Error() != "context is required" {
		t.Fatalf("expected context-required precedence error, got %v", err)
	}
	if err := NewStdioTransport(server, nil, nil).Run(context.Background()); err == nil || err.Error() != "reader is required" {
		t.Fatalf("expected reader-required precedence error, got %v", err)
	}
}

type failingWriter struct{}

func (f failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failure")
}

func TestStdioTransport_RunEOFReturnsNil(t *testing.T) {
	server := NewServer(Options{})
	transport := NewStdioTransport(server, bytes.NewBuffer(nil), bytes.NewBuffer(nil))

	if err := transport.Run(context.Background()); err != nil {
		t.Fatalf("expected EOF-only run to return nil, got %v", err)
	}
}

func TestStdioTransport_RunReturnsDecodeError(t *testing.T) {
	server := NewServer(Options{})
	transport := NewStdioTransport(server, bytes.NewBufferString("{invalid-json"), bytes.NewBuffer(nil))

	err := transport.Run(context.Background())
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "failed to decode mcp request:") {
		t.Fatalf("expected decode failure prefix, got %v", err)
	}
}

func TestStdioTransport_RunReturnsEncodeError(t *testing.T) {
	server := NewServer(Options{})
	transport := NewStdioTransport(
		server,
		bytes.NewBufferString(`{"id":"1","method":"initialize","params":{}}`),
		failingWriter{},
	)

	err := transport.Run(context.Background())
	if err == nil {
		t.Fatal("expected encode error")
	}
	if !strings.Contains(err.Error(), "failed to encode mcp response:") {
		t.Fatalf("expected encode failure prefix, got %v", err)
	}
}

func TestStdioTransport_RunContextCancellationUnblocksPipeReader(t *testing.T) {
	server := NewServer(Options{})
	reader, writer := io.Pipe()
	defer writer.Close()

	ctx, cancel := context.WithCancel(context.Background())
	transport := NewStdioTransport(server, reader, bytes.NewBuffer(nil))

	done := make(chan error, 1)
	go func() {
		done <- transport.Run(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected transport to exit promptly after context cancellation")
	}
}
