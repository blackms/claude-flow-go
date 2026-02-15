package claudeflow

import "testing"

func TestNewCompletionHandler_AllowsNilDependencies(t *testing.T) {
	handler := NewCompletionHandler(nil, nil)
	if handler == nil {
		t.Fatal("expected completion handler instance")
	}

	result := handler.Complete(nil, nil)
	if result == nil {
		t.Fatal("expected non-nil completion result")
	}
	if len(result.Values) != 0 {
		t.Fatalf("expected empty completion values, got %d", len(result.Values))
	}
}

func TestCompletionHandler_CompleteHandlesNilReceiver(t *testing.T) {
	var handler *CompletionHandler
	result := handler.Complete(nil, nil)
	if result == nil {
		t.Fatal("expected non-nil completion result")
	}
	if len(result.Values) != 0 {
		t.Fatalf("expected empty completion values, got %d", len(result.Values))
	}
}
