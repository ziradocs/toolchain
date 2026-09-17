// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

func TestMetricParserKeepsStructuredFields(t *testing.T) {
	ctx := &ParseContext{Mode: "flex", Lines: []string{"<<metric>>", "label: Revenue", "value: $42", "delta: +4%", "trend: up", "<<end>>"}}
	result := (&MetricParser{}).Parse(ctx, 0)
	m, ok := result.Element.(*ast.MetricElement)
	if !ok || m.Label != "Revenue" || m.Value != "$42" || m.Trend != "up" {
		t.Fatalf("metric parsed incorrectly: %#v", result.Element)
	}
	if result.ConsumedLines != 6 || len(result.Diagnostics) != 0 {
		t.Fatalf("unexpected parse result: %#v", result)
	}
}
