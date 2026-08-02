// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package a11y

import "testing"

func TestIsValidLangTag(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"two-letter primary subtag", "es", true},
		{"language-region", "es-MX", true},
		{"language-region lowercase region", "en-us", true},
		{"language-script-region", "zh-Hans-CN", true},
		{"language-script", "sr-Latn", true},
		{"language-variant with digit prefix", "de-DE-1996", true},
		{"three-letter primary subtag", "fil", true},
		{"eight-letter primary subtag (max)", "abcdefgh", true},
		{"single-letter primary subtag rejected", "e", false},
		{"empty string rejected", "", false},
		{"nine-letter primary subtag rejected", "abcdefghi", false},
		{"leading hyphen rejected", "-es", false},
		{"trailing hyphen rejected", "es-", false},
		{"double hyphen rejected", "es--MX", false},
		{"underscore rejected", "es_MX", false},
		{"whitespace rejected", "es MX", false},
		{"subtag over 8 chars rejected", "es-123456789", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidLangTag(tt.in); got != tt.want {
				t.Errorf("IsValidLangTag(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
