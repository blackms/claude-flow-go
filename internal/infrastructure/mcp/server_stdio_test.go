package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestStdioTransport_RunRejectsUnconfiguredServer(t *testing.T) {
	var nilTransport *StdioTransport
	if err := nilTransport.Run(context.Background()); err == nil || !errors.Is(err, shared.ErrStdioNotConfigured) {
		t.Fatalf("expected nil transport server-configured error, got %v", err)
	}

	transport := NewStdioTransport(nil, bytes.NewBuffer(nil), bytes.NewBuffer(nil))
	if err := transport.Run(context.Background()); err == nil || !errors.Is(err, shared.ErrStdioNotConfigured) {
		t.Fatalf("expected nil server configured error, got %v", err)
	}
}

func TestStdioTransport_RunRejectsNilContext(t *testing.T) {
	server := NewServer(Options{})
	transport := NewStdioTransport(server, bytes.NewBuffer(nil), bytes.NewBuffer(nil))

	if err := transport.Run(nil); err == nil || !errors.Is(err, shared.ErrContextRequired) {
		t.Fatalf("expected context-required error, got %v", err)
	}
}

func TestStdioTransport_RunRejectsNilReaderOrWriter(t *testing.T) {
	server := NewServer(Options{})

	if err := NewStdioTransport(server, nil, bytes.NewBuffer(nil)).Run(context.Background()); err == nil || !errors.Is(err, shared.ErrReaderRequired) {
		t.Fatalf("expected reader-required error, got %v", err)
	}
	if err := NewStdioTransport(server, bytes.NewBuffer(nil), nil).Run(context.Background()); err == nil || !errors.Is(err, shared.ErrWriterRequired) {
		t.Fatalf("expected writer-required error, got %v", err)
	}
	if err := NewStdioTransport(server, nil, nil).Run(context.Background()); err == nil || !errors.Is(err, shared.ErrReaderRequired) {
		t.Fatalf("expected reader-required precedence when both are nil, got %v", err)
	}
}

func TestStdioTransport_RunValidationPrecedence(t *testing.T) {
	server := NewServer(Options{})

	if err := NewStdioTransport(nil, nil, nil).Run(nil); err == nil || !errors.Is(err, shared.ErrStdioNotConfigured) {
		t.Fatalf("expected server-configured precedence error, got %v", err)
	}
	if err := NewStdioTransport(server, nil, nil).Run(nil); err == nil || !errors.Is(err, shared.ErrContextRequired) {
		t.Fatalf("expected context-required precedence error, got %v", err)
	}
	if err := NewStdioTransport(server, nil, nil).Run(context.Background()); err == nil || !errors.Is(err, shared.ErrReaderRequired) {
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

func TestStdioTransport_RunProcessesSequentialRequests(t *testing.T) {
	server := NewServer(Options{})
	input := bytes.NewBufferString(
		`{"id":"1","method":"initialize","params":{}}` + "\n" +
			`{"id":"2","method":"tools/list","params":{}}` + "\n",
	)
	output := bytes.NewBuffer(nil)

	transport := NewStdioTransport(server, input, output)
	if err := transport.Run(context.Background()); err != nil {
		t.Fatalf("expected transport run success, got %v", err)
	}

	decoder := json.NewDecoder(output)

	var first shared.MCPResponse
	if err := decoder.Decode(&first); err != nil {
		t.Fatalf("expected first response, got %v", err)
	}
	if first.ID != "1" || first.Error != nil {
		t.Fatalf("expected successful initialize response, got %+v", first)
	}

	var second shared.MCPResponse
	if err := decoder.Decode(&second); err != nil {
		t.Fatalf("expected second response, got %v", err)
	}
	if second.ID != "2" || second.Error != nil {
		t.Fatalf("expected successful tools/list response, got %+v", second)
	}
	result, ok := second.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected tools/list map result, got %T", second.Result)
	}
	if _, ok := result["tools"]; !ok {
		t.Fatalf("expected tools/list result to contain tools array, got %+v", result)
	}
}

func TestStdioTransport_RunReturnsProtocolValidationErrors(t *testing.T) {
	server := NewServer(Options{
		Tools: []shared.MCPToolProvider{
			guardTestProvider{},
		},
	})
	input := bytes.NewBufferString(
		`{"id":"bad-args","method":"tools/call","params":{"name":"guard/provider-tool","arguments":"not-an-object"}}` + "\n",
	)
	output := bytes.NewBuffer(nil)

	transport := NewStdioTransport(server, input, output)
	if err := transport.Run(context.Background()); err != nil {
		t.Fatalf("expected transport run to complete despite protocol error response, got %v", err)
	}

	var response shared.MCPResponse
	if err := json.NewDecoder(output).Decode(&response); err != nil {
		t.Fatalf("expected protocol error response, got %v", err)
	}
	if response.Error == nil {
		t.Fatal("expected protocol error response")
	}
	if response.Error.Code != -32602 {
		t.Fatalf("expected invalid params code, got %d", response.Error.Code)
	}
	if response.Error.Message != "Invalid params: arguments must be an object" {
		t.Fatalf("expected invalid arguments message, got %q", response.Error.Message)
	}
}

func TestStdioTransport_RunContinuesAfterProtocolError(t *testing.T) {
	server := NewServer(Options{})
	input := bytes.NewBufferString(
		`{"id":"bad-method","method":"   ","params":{}}` + "\n" +
			`{"id":"ok-init","method":"initialize","params":{}}` + "\n",
	)
	output := bytes.NewBuffer(nil)

	transport := NewStdioTransport(server, input, output)
	if err := transport.Run(context.Background()); err != nil {
		t.Fatalf("expected transport run success, got %v", err)
	}

	decoder := json.NewDecoder(output)

	var first shared.MCPResponse
	if err := decoder.Decode(&first); err != nil {
		t.Fatalf("expected first response, got %v", err)
	}
	if first.ID != "bad-method" {
		t.Fatalf("expected first response id bad-method, got %q", first.ID)
	}
	if first.Error == nil || first.Error.Code != -32600 || first.Error.Message != "Method is required" {
		t.Fatalf("expected invalid-request blank method error, got %+v", first.Error)
	}

	var second shared.MCPResponse
	if err := decoder.Decode(&second); err != nil {
		t.Fatalf("expected second response, got %v", err)
	}
	if second.ID != "ok-init" || second.Error != nil {
		t.Fatalf("expected successful initialize response after protocol error, got %+v", second)
	}
}
