package slices

import (
	"testing"
)

func TestFirstNotZero(t *testing.T) {
	tests := []struct {
		name     string
		slice    []any
		expected any
	}{
		{"empty slice", []any{}, nil},
		{"single non-zero value", []any{5}, 5},
		{"multiple non-zero values", []any{5, 10, 15}, 5},
		{"only zero values", []any{0, 0, 0}, 0},
		{"with nil", []any{nil, 10, 0}, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FirstNotZero(tt.slice...)
			if result != tt.expected {
				t.Errorf("Expected %d, but got %d", tt.expected, result)
			}
		})
	}
}
