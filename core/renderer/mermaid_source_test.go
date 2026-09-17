// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import "testing"

func TestPrepareMermaidContent_OnlyChangesRealFlowcharts(t *testing.T) {
	flowchart := "flowchart TD\n  A[1. Inicio] --> B"
	if got := PrepareMermaidContent(flowchart, "flowchart"); got != "flowchart TD\n  A['1. Inicio'] --> B" {
		t.Fatalf("flowchart = %q", got)
	}
	quadrant := "quadrantChart\n  x-axis Bajo --> Alto"
	if got := PrepareMermaidContent(quadrant, "quadrantchart"); got != quadrant {
		t.Fatalf("quadrant chart was rewritten: %q", got)
	}
}
