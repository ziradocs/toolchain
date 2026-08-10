// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import "testing"

// TestFrontMatterParser_TOCScalarTrue covers the shape already committed in
// 8 example files (`toc: true`/`toc: false`) — a bare bool is shorthand for
// `{enabled: <bool>}`, the same tolerance `numbering:` and
// `footer.page_numbers:` already have.
func TestFrontMatterParser_TOCScalarTrue(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\ntoc: true\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.TOC == nil || node.TOC.Enabled == nil {
		t.Fatal("TOC.Enabled should not be nil")
	}
	if !*node.TOC.Enabled {
		t.Errorf("TOC.Enabled = %v, want true", *node.TOC.Enabled)
	}
	if node.TOC.Depth != nil {
		t.Errorf("TOC.Depth = %v, want nil for a scalar toc:", *node.TOC.Depth)
	}
}

// TestFrontMatterParser_TOCScalarFalse is the symmetric case.
func TestFrontMatterParser_TOCScalarFalse(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\ntoc: false\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil || node.TOC == nil || node.TOC.Enabled == nil {
		t.Fatal("TOC.Enabled should not be nil")
	}
	if *node.TOC.Enabled {
		t.Errorf("TOC.Enabled = %v, want false", *node.TOC.Enabled)
	}
}

// TestFrontMatterParser_TOCMapFull covers the map shape `doclang init`
// emits: `toc:\n  enabled: true\n  depth: 3`.
func TestFrontMatterParser_TOCMapFull(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\ntoc:\n  enabled: true\n  depth: 3\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil || node.TOC == nil {
		t.Fatal("TOC should not be nil")
	}
	if node.TOC.Enabled == nil || !*node.TOC.Enabled {
		t.Errorf("TOC.Enabled = %v, want true", node.TOC.Enabled)
	}
	if node.TOC.Depth == nil || *node.TOC.Depth != 3 {
		t.Errorf("TOC.Depth = %v, want 3", node.TOC.Depth)
	}
}

// TestFrontMatterParser_TOCMapDepthOnly covers a map that only declares
// depth — Enabled must stay nil (no opinion), not collapse to false.
func TestFrontMatterParser_TOCMapDepthOnly(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, _ := p.Parse("---\nmode: flex\ntitle: Doc\ntoc:\n  depth: 2\n---\n\nContenido.")
	if node == nil || node.TOC == nil {
		t.Fatal("TOC should not be nil")
	}
	if node.TOC.Enabled != nil {
		t.Errorf("TOC.Enabled = %v, want nil when only depth is declared", *node.TOC.Enabled)
	}
	if node.TOC.Depth == nil || *node.TOC.Depth != 2 {
		t.Errorf("TOC.Depth = %v, want 2", node.TOC.Depth)
	}
}

// TestFrontMatterParser_TOCMapEmpty collapses to nil, same criterion as
// `numbering: {}`.
func TestFrontMatterParser_TOCMapEmpty(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, _ := p.Parse("---\nmode: flex\ntitle: Doc\ntoc: {}\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.TOC != nil {
		t.Errorf("TOC = %+v, want nil for an empty map", node.TOC)
	}
}

// TestFrontMatterParser_TOCAbsent covers the "not declared at all" case.
func TestFrontMatterParser_TOCAbsent(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, _ := p.Parse("---\nmode: flex\ntitle: Doc\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.TOC != nil {
		t.Errorf("TOC = %+v, want nil when not declared", node.TOC)
	}
}

// TestFrontMatterParser_TOCImplicitNull covers `toc:` declared with no
// value at all — a stray/leftover key, not an intentional empty map. Same
// yaml.v3 behavior as rawHeaderFooterText.Text (see
// TestFrontMatterParser_HeaderTextAbsent in frontmatter_headerfooter_test.go):
// a null scalar into a pointer field leaves the pointer nil without even
// invoking UnmarshalYAML, so this must produce neither a TOC value nor a
// diagnostic — confirmed empirically for rawTOC specifically (not just
// assumed from the header/footer precedent) during implementation.
func TestFrontMatterParser_TOCImplicitNull(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\ntoc:\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
		if d.RuleID == "FRONT005" {
			t.Errorf("unexpected FRONT005 warning for an empty toc: line, got: %v", d)
		}
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.TOC != nil {
		t.Errorf("TOC = %+v, want nil for `toc:` with no value", node.TOC)
	}
}

// TestFrontMatterParser_TOCScalarNonBool covers a scalar that parses as
// neither true nor false (`toc: sí`) — degrades to a FRONT005 warning with
// TOC left nil, same "bad value never kills the front matter" contract as
// the bad-shape tests above, but for a bad VALUE within an otherwise valid
// shape (ScalarNode).
func TestFrontMatterParser_TOCScalarNonBool(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\ntoc: sí\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.TOC != nil {
		t.Errorf("TOC = %+v, want nil for a non-bool scalar", node.TOC)
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT005" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT005 warning for `toc: sí`, got: %+v", diags)
	}
}

// TestFrontMatterParser_TOCDepthQuoted covers the hazard called out during
// design: a quoted depth (`depth: "3"`) must NOT abort the front matter —
// it must degrade to a warning with Depth left nil.
func TestFrontMatterParser_TOCDepthQuoted(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\ntoc:\n  depth: \"3\"\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil || node.Title != "Doc" {
		t.Fatal("front matter should survive a quoted toc.depth")
	}
	if node.TOC != nil && node.TOC.Depth != nil {
		t.Errorf("TOC.Depth = %v, want nil for a quoted depth", *node.TOC.Depth)
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT005" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT005 warning for a quoted toc.depth, got: %+v", diags)
	}
}

// TestFrontMatterParser_TOCDepthOutOfRange covers depth outside [1,6]: the
// value is conserved (extractSubsections/its call sites are total and just
// guard with `> 1`, nothing to break), with a warning.
func TestFrontMatterParser_TOCDepthOutOfRange(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\ntoc:\n  depth: 12\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil || node.TOC == nil || node.TOC.Depth == nil || *node.TOC.Depth != 12 {
		t.Fatalf("TOC.Depth should be conserved as 12, got: %+v", node.TOC)
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT005" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT005 warning for an out-of-range toc.depth, got: %+v", diags)
	}
}

// TestFrontMatterParser_TOCUnknownKeyWarns covers a code review finding: a
// typo'd map key (`enable` instead of `enabled`) used to decode into an
// all-nil rawTOC with zero diagnostics — the same silent-drop failure mode
// issue #121 tracks, just inside an already-modeled key. The typo must
// surface as a FRONT005 warning, and a sibling correctly-spelled key in the
// same map must still take effect.
func TestFrontMatterParser_TOCUnknownKeyWarns(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\ntoc:\n  enable: false\n  depth: 2\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil || node.TOC == nil {
		t.Fatal("TOC should not be nil — the correctly-spelled 'depth' key must still apply")
	}
	if node.TOC.Enabled != nil {
		t.Errorf("TOC.Enabled = %v, want nil — 'enable' is a typo, not 'enabled'", *node.TOC.Enabled)
	}
	if node.TOC.Depth == nil || *node.TOC.Depth != 2 {
		t.Errorf("TOC.Depth = %v, want 2", node.TOC.Depth)
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT005" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT005 warning for the unrecognized 'enable' key, got: %+v", diags)
	}
}

// TestFrontMatterParser_TOCBadShapeDoesNotKillFrontMatter is the contract
// test: a `toc:` value that is neither a scalar nor a map (a YAML sequence)
// must not take down the rest of a valid document. This fixes the blast
// radius that modeling `toc:` as a known key would otherwise introduce
// (see frontmatter.go's rawTOC doc comment) — the exact failure class issue
// #115 was about, one level up.
func TestFrontMatterParser_TOCBadShapeDoesNotKillFrontMatter(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nauthor: Someone\ntoc:\n  - a\n  - b\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil — a bad toc: shape must not kill the front matter")
	}
	if node.Title != "Doc" || node.Author != "Someone" {
		t.Errorf("front matter fields should survive a bad toc: shape, got Title=%q Author=%q", node.Title, node.Author)
	}
	if node.TOC != nil {
		t.Errorf("TOC = %+v, want nil for a bad shape", node.TOC)
	}
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("expected no error-severity diagnostics, got: %v", d)
		}
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT005" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT005 warning for a sequence toc:, got: %+v", diags)
	}
}
