// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import "testing"

// TestFrontMatterParser_HeaderTextScalar covers issue #115: `doclang init`'s
// `report` template emitted `header:\n  text: <name>` (a bare scalar), which
// `rawHeaderConfig.Text`'s map-only shape (`left`/`center`/`right`) rejected
// outright. A scalar is now accepted as shorthand for `center` — the same
// tolerant-parser precedent #100 set for `numbering:`.
func TestFrontMatterParser_HeaderTextScalar(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nheader:\n  enabled: true\n  text: Sample Report\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.HeaderFooter == nil || node.HeaderFooter.Header == nil || node.HeaderFooter.Header.Text == nil {
		t.Fatal("HeaderFooter.Header.Text should not be nil")
	}
	text := node.HeaderFooter.Header.Text
	if text.Center != "Sample Report" {
		t.Errorf("Center = %q, want %q", text.Center, "Sample Report")
	}
	if text.Left != "" || text.Right != "" {
		t.Errorf("Left/Right should stay empty, got Left=%q Right=%q", text.Left, text.Right)
	}
}

// TestFrontMatterParser_HeaderTextMap covers the existing, still-supported
// map shape — the scalar shorthand must not regress it.
func TestFrontMatterParser_HeaderTextMap(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nheader:\n  enabled: true\n  text:\n    left: L\n    center: C\n    right: R\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	text := node.HeaderFooter.Header.Text
	if text == nil {
		t.Fatal("Text should not be nil")
	}
	if text.Left != "L" || text.Center != "C" || text.Right != "R" {
		t.Errorf("Text = %+v, want {Left:L Center:C Right:R}", text)
	}
}

// TestFrontMatterParser_FooterTextScalar proves the scalar shorthand applies
// to footer.text too, not just header.text — both share the same
// rawHeaderFooterText.UnmarshalYAML.
func TestFrontMatterParser_FooterTextScalar(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nfooter:\n  enabled: true\n  text: Page footer\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.HeaderFooter == nil || node.HeaderFooter.Footer == nil || node.HeaderFooter.Footer.Text == nil {
		t.Fatal("HeaderFooter.Footer.Text should not be nil")
	}
	if node.HeaderFooter.Footer.Text.Center != "Page footer" {
		t.Errorf("Center = %q, want %q", node.HeaderFooter.Footer.Text.Center, "Page footer")
	}
}

// TestFrontMatterParser_LayoutDefaultsHeaderTextScalar proves the shorthand
// also applies under layout_defaults, the fourth and last call site of
// rawHeaderFooterText.
func TestFrontMatterParser_LayoutDefaultsHeaderTextScalar(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nlayout_defaults:\n  title:\n    header:\n      enabled: true\n      text: Title slide header\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	layout, ok := node.HeaderFooter.LayoutDefaults["title"]
	if !ok || layout == nil || layout.Header == nil || layout.Header.Text == nil {
		t.Fatal("layout_defaults.title.header.text should not be nil")
	}
	if layout.Header.Text.Center != "Title slide header" {
		t.Errorf("Center = %q, want %q", layout.Header.Text.Center, "Title slide header")
	}
}

// TestFrontMatterParser_HeaderTextAbsent covers the case where `text:` is
// declared but empty (or `header:` omits it entirely) — no diagnostic, no
// error, and no half-populated Text (yaml.v3 leaves a *pointer* field nil
// for an empty/null scalar without even invoking UnmarshalYAML, confirmed
// against this parser's yaml library during implementation).
func TestFrontMatterParser_HeaderTextAbsent(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nheader:\n  enabled: true\n  text:\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.HeaderFooter.Header.Text != nil {
		t.Errorf("Text = %+v, want nil when text: is empty", node.HeaderFooter.Header.Text)
	}
}

// TestFrontMatterParser_HeaderTextInvalidSequence covers the rejection path:
// a shape that is neither a scalar nor a map (a YAML sequence) must still
// produce a hard parse error, not be silently coerced.
func TestFrontMatterParser_HeaderTextInvalidSequence(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nheader:\n  enabled: true\n  text:\n    - a\n    - b\n---\n\nContenido.")
	if node != nil {
		t.Fatal("node should be nil for invalid YAML")
	}
	foundError := false
	for _, d := range diags {
		if d.IsError() {
			foundError = true
		}
	}
	if !foundError {
		t.Errorf("expected an error-severity diagnostic for a sequence text:, got: %+v", diags)
	}
}

// TestFrontMatterParser_FooterPageNumbersBoolTrue covers issue #115's second
// bug: `doclang init`'s `report` template emitted `footer:\n  page-numbers:
// true` (wrong key, wrong shape). The fixed template now emits
// `page_numbers:\n  enabled: true`, but the parser also accepts a bare bool
// as shorthand for `{enabled: <bool>}`, mirroring `numbering:`'s tolerance.
func TestFrontMatterParser_FooterPageNumbersBoolTrue(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nfooter:\n  enabled: true\n  page_numbers: true\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	pn := node.HeaderFooter.Footer.PageNumbers
	if pn == nil {
		t.Fatal("PageNumbers should not be nil")
	}
	if !pn.Enabled {
		t.Errorf("Enabled = %v, want true", pn.Enabled)
	}
}

// TestFrontMatterParser_FooterPageNumbersBoolFalse is the asymmetric case
// that matters: page_numbers.Enabled is a plain bool (not *bool, unlike
// FrontMatterNode.Numbering), so `page_numbers: false` must still produce a
// non-nil PageNumbers with Enabled=false — distinguishable from the key
// being absent entirely, which leaves PageNumbers nil (see the "absent"
// test below). Both mean "no page numbers" downstream in slidelang's
// template (`{{if $finalFooterConfig.PageNumbers}}` then `.Enabled`), so
// the distinction is harmless there, but the parser must not collapse them.
func TestFrontMatterParser_FooterPageNumbersBoolFalse(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nfooter:\n  enabled: true\n  page_numbers: false\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	pn := node.HeaderFooter.Footer.PageNumbers
	if pn == nil {
		t.Fatal("PageNumbers should not be nil for an explicit page_numbers: false")
	}
	if pn.Enabled {
		t.Errorf("Enabled = %v, want false", pn.Enabled)
	}
}

// TestFrontMatterParser_FooterPageNumbersMap covers the pre-existing map
// shape, unaffected by the bool shorthand.
func TestFrontMatterParser_FooterPageNumbersMap(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nfooter:\n  enabled: true\n  page_numbers:\n    enabled: true\n    format: \"{current} / {total}\"\n    position: right\n    exclude_title_slides: true\n    exclude_closing_slides: true\n    start_from: 1\n    style: default\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	pn := node.HeaderFooter.Footer.PageNumbers
	if pn == nil {
		t.Fatal("PageNumbers should not be nil")
	}
	if !pn.Enabled || pn.Format != "{current} / {total}" || pn.Position != "right" ||
		!pn.ExcludeTitleSlides || !pn.ExcludeClosingSlides || pn.StartFrom != 1 || pn.Style != "default" {
		t.Errorf("PageNumbers = %+v, fields did not round-trip", pn)
	}
}

// TestFrontMatterParser_FooterPageNumbersAbsent covers the "not declared at
// all" case, which must leave PageNumbers nil.
func TestFrontMatterParser_FooterPageNumbersAbsent(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nfooter:\n  enabled: true\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.HeaderFooter.Footer.PageNumbers != nil {
		t.Errorf("PageNumbers = %+v, want nil when page_numbers: is not declared", node.HeaderFooter.Footer.PageNumbers)
	}
}

// TestFrontMatterParser_FooterPageNumbersInvalidSequence covers the
// rejection path for page_numbers, mirroring
// TestFrontMatterParser_HeaderTextInvalidSequence.
func TestFrontMatterParser_FooterPageNumbersInvalidSequence(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nfooter:\n  enabled: true\n  page_numbers:\n    - a\n    - b\n---\n\nContenido.")
	if node != nil {
		t.Fatal("node should be nil for invalid YAML")
	}
	foundError := false
	for _, d := range diags {
		if d.IsError() {
			foundError = true
		}
	}
	if !foundError {
		t.Errorf("expected an error-severity diagnostic for a sequence page_numbers:, got: %+v", diags)
	}
}
