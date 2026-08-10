// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import "testing"

// TestFrontMatterParser_PageScalar covers `page: A4`, the shorthand for
// `page: {size: A4}` — necessary even though no known template emits it:
// without it, this form would go from silently-inert to a hard parse
// failure the moment `page:` becomes a known key.
func TestFrontMatterParser_PageScalar(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\npage: A4\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil || node.Page == nil {
		t.Fatal("Page should not be nil")
	}
	if node.Page.Size != "A4" {
		t.Errorf("Page.Size = %q, want %q", node.Page.Size, "A4")
	}
	if node.Page.Margins != nil {
		t.Errorf("Page.Margins = %+v, want nil for a scalar page:", node.Page.Margins)
	}
}

// TestFrontMatterParser_PageMapSizeAndMarginsScalar covers the shape
// `doclang init`'s `technical` template emits: `page:\n  size: A4\n
// margins: 2cm` — margins as a scalar fills all four sides.
func TestFrontMatterParser_PageMapSizeAndMarginsScalar(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\npage:\n  size: A4\n  margins: 2cm\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil || node.Page == nil {
		t.Fatal("Page should not be nil")
	}
	if node.Page.Size != "A4" {
		t.Errorf("Page.Size = %q, want %q", node.Page.Size, "A4")
	}
	if node.Page.Margins == nil {
		t.Fatal("Page.Margins should not be nil")
	}
	want := "2cm"
	if node.Page.Margins.Top != want || node.Page.Margins.Right != want || node.Page.Margins.Bottom != want || node.Page.Margins.Left != want {
		t.Errorf("Page.Margins = %+v, want all sides %q", node.Page.Margins, want)
	}
	// Verbatim, not resolved to inches — this is the contract the parser
	// must never break (see convertPage's doc comment).
	if node.Page.Margins.Top == "0.7874" {
		t.Fatal("Page.Margins.Top must stay the author's raw string, not a resolved inch value")
	}
}

// TestFrontMatterParser_PageMarginsPerSide covers the per-side map form.
func TestFrontMatterParser_PageMarginsPerSide(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\npage:\n  margins:\n    top: 1in\n    left: 2cm\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil || node.Page == nil || node.Page.Margins == nil {
		t.Fatal("Page.Margins should not be nil")
	}
	if node.Page.Margins.Top != "1in" {
		t.Errorf("Page.Margins.Top = %q, want %q", node.Page.Margins.Top, "1in")
	}
	if node.Page.Margins.Left != "2cm" {
		t.Errorf("Page.Margins.Left = %q, want %q", node.Page.Margins.Left, "2cm")
	}
	if node.Page.Margins.Right != "" || node.Page.Margins.Bottom != "" {
		t.Errorf("undeclared sides should stay empty, got Right=%q Bottom=%q", node.Page.Margins.Right, node.Page.Margins.Bottom)
	}
}

// TestFrontMatterParser_PageMapEmpty collapses to nil.
func TestFrontMatterParser_PageMapEmpty(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, _ := p.Parse("---\nmode: flex\ntitle: Doc\npage: {}\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.Page != nil {
		t.Errorf("Page = %+v, want nil for an empty map", node.Page)
	}
}

// TestFrontMatterParser_PageAbsent covers the "not declared" case.
func TestFrontMatterParser_PageAbsent(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, _ := p.Parse("---\nmode: flex\ntitle: Doc\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.Page != nil {
		t.Errorf("Page = %+v, want nil when not declared", node.Page)
	}
}

// TestFrontMatterParser_PageImplicitNull mirrors
// TestFrontMatterParser_TOCImplicitNull for `page:` — a stray `page:` line
// with no value must produce neither a Page value nor a diagnostic.
func TestFrontMatterParser_PageImplicitNull(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\npage:\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
		if d.RuleID == "FRONT006" {
			t.Errorf("unexpected FRONT006 warning for an empty page: line, got: %v", d)
		}
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.Page != nil {
		t.Errorf("Page = %+v, want nil for `page:` with no value", node.Page)
	}
}

// TestFrontMatterParser_PageUnknownSize covers an unrecognized size name
// ("Carta" instead of "Letter"): conserved verbatim, with a warning — the
// renderer decides the fallback (see core/util.PaperSizeInches), the parser
// doesn't silently substitute one.
func TestFrontMatterParser_PageUnknownSize(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\npage:\n  size: Carta\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil || node.Page == nil || node.Page.Size != "Carta" {
		t.Fatalf("Page.Size should be conserved as %q, got: %+v", "Carta", node.Page)
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT006" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT006 warning for an unrecognized page.size, got: %+v", diags)
	}
}

// TestFrontMatterParser_PageMarginsUnrecognizedUnit covers an unrecognized
// length unit: conserved, with a warning.
func TestFrontMatterParser_PageMarginsUnrecognizedUnit(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\npage:\n  margins: 5furlongs\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil || node.Page == nil || node.Page.Margins == nil || node.Page.Margins.Top != "5furlongs" {
		t.Fatalf("Page.Margins should be conserved as %q, got: %+v", "5furlongs", node.Page)
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT006" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT006 warning for an unrecognized margin unit, got: %+v", diags)
	}
}

// TestFrontMatterParser_PageUnknownKeyWarns mirrors
// TestFrontMatterParser_TOCUnknownKeyWarns for `page:` — the singular typo
// "margin" (for "margins") used to decode into an all-nil rawPage with no
// diagnostic at all.
func TestFrontMatterParser_PageUnknownKeyWarns(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\npage:\n  size: A4\n  margin: 2cm\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil || node.Page == nil || node.Page.Size != "A4" {
		t.Fatal("Page.Size should still apply from the correctly-spelled 'size' key")
	}
	if node.Page.Margins != nil {
		t.Errorf("Page.Margins = %+v, want nil — 'margin' is a typo, not 'margins'", node.Page.Margins)
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT006" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT006 warning for the unrecognized 'margin' key, got: %+v", diags)
	}
}

// TestFrontMatterParser_PageMarginsUnknownSideWarns covers the same typo
// class one level deeper: an unrecognized side key inside `page.margins:`
// (e.g. "topp") used to be silently dropped by a tagged-struct decode.
func TestFrontMatterParser_PageMarginsUnknownSideWarns(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\npage:\n  margins:\n    topp: 1in\n    left: 2cm\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil || node.Page == nil || node.Page.Margins == nil {
		t.Fatal("Page.Margins should not be nil — the correctly-spelled 'left' key must still apply")
	}
	if node.Page.Margins.Top != "" {
		t.Errorf("Page.Margins.Top = %q, want empty — 'topp' is a typo, not 'top'", node.Page.Margins.Top)
	}
	if node.Page.Margins.Left != "2cm" {
		t.Errorf("Page.Margins.Left = %q, want %q", node.Page.Margins.Left, "2cm")
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT006" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT006 warning for the unrecognized 'topp' margin side, got: %+v", diags)
	}
}

// TestFrontMatterParser_PageBadShapeDoesNotKillFrontMatter mirrors
// TestFrontMatterParser_TOCBadShapeDoesNotKillFrontMatter for `page:`.
func TestFrontMatterParser_PageBadShapeDoesNotKillFrontMatter(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nauthor: Someone\npage:\n  - a\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil — a bad page: shape must not kill the front matter")
	}
	if node.Title != "Doc" || node.Author != "Someone" {
		t.Errorf("front matter fields should survive a bad page: shape, got Title=%q Author=%q", node.Title, node.Author)
	}
	if node.Page != nil {
		t.Errorf("Page = %+v, want nil for a bad shape", node.Page)
	}
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("expected no error-severity diagnostics, got: %v", d)
		}
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT006" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT006 warning for a sequence page:, got: %+v", diags)
	}
}
