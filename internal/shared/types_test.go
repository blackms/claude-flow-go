package shared

import "testing"

func TestIntToPriority(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected TaskPriority
	}{
		{name: "very low", input: 0, expected: PriorityLow},
		{name: "low upper bound", input: 3, expected: PriorityLow},
		{name: "medium lower bound", input: 4, expected: PriorityMedium},
		{name: "medium middle", input: 6, expected: PriorityMedium},
		{name: "high lower bound", input: 8, expected: PriorityHigh},
		{name: "high upper bound", input: 10, expected: PriorityHigh},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IntToPriority(tt.input)
			if got != tt.expected {
				t.Fatalf("IntToPriority(%d) = %q, expected %q", tt.input, got, tt.expected)
			}
		})
	}
}
