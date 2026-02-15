package tasks

import (
	"testing"

	"github.com/anthropics/claude-flow-go/internal/shared"
)

func TestManagedPriorityToTaskPriority(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected shared.TaskPriority
	}{
		{name: "low range", input: 1, expected: shared.PriorityLow},
		{name: "medium boundary", input: 4, expected: shared.PriorityMedium},
		{name: "medium range", input: 7, expected: shared.PriorityMedium},
		{name: "high boundary", input: 8, expected: shared.PriorityHigh},
		{name: "high range", input: 10, expected: shared.PriorityHigh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := managedPriorityToTaskPriority(tt.input)
			if got != tt.expected {
				t.Fatalf("managedPriorityToTaskPriority(%d) = %q, expected %q", tt.input, got, tt.expected)
			}
		})
	}
}
