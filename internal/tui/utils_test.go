package tui

import (
	"testing"
)

func TestStringsRepeat(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		count    int
		expected string
	}{
		{
			name:     "single char",
			s:        "a",
			count:    3,
			expected: "aaa",
		},
		{
			name:     "multiple chars",
			s:        "ab",
			count:    3,
			expected: "ababab",
		},
		{
			name:     "zero count",
			s:        "test",
			count:    0,
			expected: "",
		},
		{
			name:     "empty string",
			s:        "",
			count:    5,
			expected: "",
		},
		{
			name:     "single repeat",
			s:        "x",
			count:    1,
			expected: "x",
		},
		{
			name:     "space repeat",
			s:        " ",
			count:    4,
			expected: "    ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringsRepeat(tt.s, tt.count)
			if result != tt.expected {
				t.Errorf("StringsRepeat(%q, %d) = %q, want %q", tt.s, tt.count, result, tt.expected)
			}
		})
	}
}
