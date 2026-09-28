// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func TestQuizPollResultsHTML(t *testing.T) {
	poll := ast.NewPollElement(diagnostics.NewPosition(1, 1))
	poll.Question, poll.Options, poll.Results = "Q?", []string{"A", "B"}, []float64{40, 60.5}
	n := 1
	poll.Responses = &n
	html := RenderElementToHTML(poll, nil, nil)
	for _, want := range []string{`<span class="result" data-percent="40">40%</span>`, `60.5%`, `<p class="responses" data-responses="1">1 response</p>`} {
		if !strings.Contains(html, want) {
			t.Fatalf("missing %q in %s", want, html)
		}
	}
	poll.Results, poll.Responses = nil, nil
	if html := RenderElementToHTML(poll, nil, nil); strings.Contains(html, "result") || strings.Contains(html, "responses") {
		t.Fatalf("legacy poll changed: %s", html)
	}
}
