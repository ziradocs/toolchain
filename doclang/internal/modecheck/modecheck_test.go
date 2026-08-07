// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package modecheck

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func newFM(mode string) *ast.FrontMatterNode {
	fm := ast.NewFrontMatterNode(diagnostics.NewPosition(1, 1))
	fm.Mode = mode
	return fm
}

func TestCheck_RejectsStrict(t *testing.T) {
	diags := Check(newFM("strict"))
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic for mode: strict, got %d", len(diags))
	}
	d := diags[0]
	if !d.IsError() {
		t.Errorf("expected severity Error (a strict document must not build as flex), got %q", d.Severity)
	}
	if d.RuleID != RuleID {
		t.Errorf("expected RuleID %q, got %q", RuleID, d.RuleID)
	}
	if d.Source != diagnosticSource {
		t.Errorf("expected Source %q, got %q", diagnosticSource, d.Source)
	}
}

// Los otros modos son la superficie de compatibilidad: el corpus de
// examples/ usa "flex" y el backfill del FrontMatterParser produce "auto".
// Ninguno debe verse afectado por este chequeo.
func TestCheck_AllowsEveryOtherMode(t *testing.T) {
	for _, mode := range []string{"flex", "flex-full", "flex-ai", "auto", ""} {
		if diags := Check(newFM(mode)); diags != nil {
			t.Errorf("mode %q: expected no diagnostic, got %v", mode, diags)
		}
	}
}

// Un .doclang sin frontmatter es válido (DocumentFlexParser lo tolera) y no
// puede haber declarado ningún modo.
func TestCheck_NilFrontMatter(t *testing.T) {
	if diags := Check(nil); diags != nil {
		t.Errorf("expected no diagnostic for a nil frontmatter, got %v", diags)
	}
}

func TestCheckAST_NilAST(t *testing.T) {
	if diags := CheckAST(nil); diags != nil {
		t.Errorf("expected no diagnostic for a nil AST, got %v", diags)
	}
}

func TestCheckAST_ReadsFrontMatter(t *testing.T) {
	doc := ast.NewAST(diagnostics.NewPosition(1, 1))
	doc.FrontMatter = newFM("strict")
	if len(CheckAST(doc)) != 1 {
		t.Error("expected CheckAST to surface the same diagnostic as Check")
	}
}
