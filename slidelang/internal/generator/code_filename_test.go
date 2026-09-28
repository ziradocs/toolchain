//go:build !js

// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

func TestCodeFilenameInSlideHTML(t *testing.T) {
	doc, diags := parser.New(util.NewNoop()).Parse("---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n  CODE typescript renewals.ts\n    const x = 1;\n", "deck.slidelang")
	for _, d := range diags {
		if d.IsError() {
			t.Fatal(d)
		}
	}
	dir := t.TempDir()
	if err := New(util.NewNoop()).GenerateWithOptions(doc, "html", dir, GeneratorOptions{AssetRoot: dir}, renderer.NewDefaultRenderContext()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "deck.html"))
	if err != nil {
		entries, _ := os.ReadDir(dir)
		t.Fatalf("read html: %v (dir: %v)", err, entries)
	}
	html := string(data)
	if !strings.Contains(html, `<div class="slidelang-filename">renewals.ts</div>`) {
		i := strings.Index(html, "slidelang-code")
		t.Fatalf("slide HTML lacks the code filename; near code: %q", html[max(0, i-50):min(len(html), i+400)])
	}
	var code *ast.CodeElement
	_ = ast.Walk(doc, func(n ast.Node) error {
		if c, ok := n.(*ast.CodeElement); ok {
			code = c
		}
		return nil
	})
	if code == nil || code.Filename != "renewals.ts" {
		t.Fatalf("code = %#v", code)
	}
}
