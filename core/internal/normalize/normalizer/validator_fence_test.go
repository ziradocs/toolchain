// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package normalizer

import (
	"strings"
	"testing"
)

// validateChartBlock cierra el bloque con la lectura del parser (fence.End):
// una línea ```` cierra un ```chart.
func TestValidator_ChartBlockClosedByLongerFence(t *testing.T) {
	var result ValidationResult
	NewValidator(false).validateChartBlock(strings.Split("```chart\ntype: bar\ntitle: T\n````", "\n"), 0, &result)
	if len(result.Errors) != 0 || len(result.Warnings) != 0 {
		t.Errorf("errors %v warnings %v", result.Errors, result.Warnings)
	}
}
