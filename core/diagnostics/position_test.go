// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package diagnostics

import "testing"

func TestNewPosition(t *testing.T) {
	pos := NewPosition(10, 5)

	if pos.Line != 10 {
		t.Errorf("Line = %d, want 10", pos.Line)
	}

	if pos.Column != 5 {
		t.Errorf("Column = %d, want 5", pos.Column)
	}
}

func TestPosition_String(t *testing.T) {
	tests := []struct {
		line     int
		column   int
		expected string
	}{
		{1, 1, "1:1"},
		{10, 5, "10:5"},
		{100, 50, "100:50"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			pos := NewPosition(tt.line, tt.column)
			result := pos.String()
			if result != tt.expected {
				t.Errorf("String() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestPosition_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		line     int
		column   int
		expected bool
	}{
		{"Valid position", 1, 1, true},
		{"Valid position 2", 10, 5, true},
		{"Invalid line (zero)", 0, 1, false},
		{"Invalid line (negative)", -1, 1, false},
		{"Invalid column (zero)", 1, 0, false},
		{"Invalid column (negative)", 1, -1, false},
		{"Both invalid", 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := Position{Line: tt.line, Column: tt.column}
			result := pos.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, want %v for position %v", result, tt.expected, pos)
			}
		})
	}
}

func TestPosition_Before(t *testing.T) {
	tests := []struct {
		name     string
		pos      Position
		other    Position
		expected bool
	}{
		{
			name:     "Before on same line",
			pos:      NewPosition(5, 10),
			other:    NewPosition(5, 20),
			expected: true,
		},
		{
			name:     "After on same line",
			pos:      NewPosition(5, 20),
			other:    NewPosition(5, 10),
			expected: false,
		},
		{
			name:     "Before on different line",
			pos:      NewPosition(5, 10),
			other:    NewPosition(10, 5),
			expected: true,
		},
		{
			name:     "After on different line",
			pos:      NewPosition(10, 5),
			other:    NewPosition(5, 10),
			expected: false,
		},
		{
			name:     "Same position",
			pos:      NewPosition(5, 10),
			other:    NewPosition(5, 10),
			expected: false,
		},
		{
			name:     "Earlier line, later column",
			pos:      NewPosition(3, 50),
			other:    NewPosition(5, 10),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pos.Before(tt.other)
			if result != tt.expected {
				t.Errorf("Before() = %v, want %v for %v before %v",
					result, tt.expected, tt.pos, tt.other)
			}
		})
	}
}
