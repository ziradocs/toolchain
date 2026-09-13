// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package content

import (
	"testing"

	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
)

func TestTitleSubtitleRule_AppliesTo(t *testing.T) {
	rule := NewTitleSubtitleRule()

	tests := []struct {
		dialect base.Dialect
		want    bool
	}{
		{base.DialectAny, true},
		{base.DialectSlides, true},
		{base.DialectDocuments, false},
	}

	for _, tt := range tests {
		if got := rule.AppliesTo(tt.dialect); got != tt.want {
			t.Errorf("AppliesTo(%v) = %v, want %v", tt.dialect, got, tt.want)
		}
	}
}
