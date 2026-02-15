package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestServer_HTTPHandleRequestRejectsNonPOST(t *testing.T) {
	server := NewServer(Options{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)

	server.handleRequest(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 for non-POST handleRequest, got %d", recorder.Code)
	}
}

func TestServer_HTTPHandleRequestReturnsParseErrorForInvalidJSON(t *testing.T) {
	server := NewServer(Options{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{invalid-json"))

	server.handleRequest(recorder, request)

	var response shared.MCPResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected parse-error response body, got %v", err)
	}
	if response.Error == nil {
		t.Fatal("expected parse error response")
	}
	if response.Error.Code != -32700 || response.Error.Message != "Parse error" {
		t.Fatalf("expected parse error payload, got %+v", response.Error)
	}
}

func TestServer_HTTPHandleRequestInitializeRoundTrip(t *testing.T) {
	server := NewServer(Options{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/",
		bytes.NewBufferString(`{"id":"http-init","method":"initialize","params":{}}`),
	)

	server.handleRequest(recorder, request)

	var response shared.MCPResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected initialize response body, got %v", err)
	}
	if response.ID != "http-init" || response.Error != nil {
		t.Fatalf("expected successful initialize response, got %+v", response)
	}
	result, ok := response.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected initialize map result, got %T", response.Result)
	}
	if result["protocolVersion"] == "" {
		t.Fatalf("expected protocolVersion in initialize response, got %+v", result)
	}
}

func TestServer_HTTPHandleListToolsReturnsSortedTools(t *testing.T) {
	server := NewServer(Options{})
	server.RegisterTool(shared.MCPTool{
		Name:        "guard/http-z",
		Description: "http z",
		Parameters:  map[string]interface{}{"type": "object"},
	})
	server.RegisterTool(shared.MCPTool{
		Name:        "guard/http-a",
		Description: "http a",
		Parameters:  map[string]interface{}{"type": "object"},
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/tools", nil)
	server.handleListTools(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for GET /tools, got %d", recorder.Code)
	}

	var payload map[string][]shared.MCPTool
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected /tools response payload, got %v", err)
	}
	tools := payload["tools"]
	if len(tools) == 0 {
		t.Fatal("expected /tools to return tool list")
	}

	var sawA, sawZ bool
	prev := ""
	for i, tool := range tools {
		if tool.Name == "" {
			t.Fatalf("expected non-empty tool name at index %d", i)
		}
		if prev != "" && prev > tool.Name {
			t.Fatalf("expected sorted tools in /tools response, got %q before %q", prev, tool.Name)
		}
		if tool.Name == "guard/http-a" {
			sawA = true
		}
		if tool.Name == "guard/http-z" {
			sawZ = true
		}
		prev = tool.Name
	}
	if !sawA || !sawZ {
		t.Fatalf("expected custom tools in /tools response, sawA=%v sawZ=%v", sawA, sawZ)
	}
}
