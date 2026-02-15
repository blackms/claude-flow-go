package mcp

import (
	"bytes"
	"context"
	"testing"
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
