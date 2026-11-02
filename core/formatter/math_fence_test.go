// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// TestFormatDocument_MathFence: `doclang fmt` canoniza un ```math a
// <<math>>, igual que $$, y el resultado vuelve a ser la misma ecuación.
func TestFormatDocument_MathFence(t *testing.T) {
	src := "# Informe\n\n```math\n\\int_0^1 x^2 dx = \\frac{1}{3}\n```\n"
	doc, _ := parser.New(util.NewNoop()).ParseDocument(src, "")
	out, err := FormatDocument(doc)
	if err != nil {
		t.Fatalf("FormatDocument: %v", err)
	}
	if !strings.Contains(out, "<<math>>\n  \\int_0^1 x^2 dx = \\frac{1}{3}\n<<end>>") {
		t.Fatalf("fmt did not canonicalize the fence to <<math>>:\n%q", out)
	}
	again, err := FormatDocument(mustParseDocument(t, out))
	if err != nil {
		t.Fatalf("FormatDocument (second pass): %v", err)
	}
	if again != out {
		t.Errorf("fmt is not idempotent on the canonical form:\n%s\n---\n%s", out, again)
	}
}

func mustParseDocument(t *testing.T, src string) *ast.AST {
	t.Helper()
	doc, _ := parser.New(util.NewNoop()).ParseDocument(src, "")
	return doc
}
