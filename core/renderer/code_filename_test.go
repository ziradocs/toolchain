// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func TestCodeFilenameHTML(t *testing.T) {
	code := ast.NewCodeElement(diagnostics.NewPosition(1, 1), "ts", "x")
	plain := RenderElementToHTML(code, nil, nil)
	code.Filename = "<b>a.ts"
	html := RenderElementToHTML(code, nil, nil)
	if !strings.Contains(html, `<figcaption class="code-filename">&lt;b&gt;a.ts</figcaption>`) || !strings.Contains(html, plain) {
		t.Fatalf("html = %s", html)
	}
}
