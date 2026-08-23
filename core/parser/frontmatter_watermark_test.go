// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import "testing"

// TestFrontMatterParser_WatermarkScalar covers `watermark: "BORRADOR"`, the
// shorthand for `watermark: {enabled: true, text: "BORRADOR"}`.
func TestFrontMatterParser_WatermarkScalar(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nwatermark: BORRADOR\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil || node.Watermark == nil {
		t.Fatal("Watermark should not be nil")
	}
	if !node.Watermark.Enabled {
		t.Error("Watermark.Enabled = false, want true for a scalar watermark:")
	}
	if node.Watermark.Text != "BORRADOR" {
		t.Errorf("Watermark.Text = %q, want %q", node.Watermark.Text, "BORRADOR")
	}
}

// TestFrontMatterParser_WatermarkMapImplicitEnabled covers a map form with
// no explicit `enabled:` key — declaring the block at all is the signal of
// intent, same as the scalar shorthand (see convertWatermark's doc
// comment for why this differs from HeaderConfig.Enabled's plain-bool
// default-false behavior).
func TestFrontMatterParser_WatermarkMapImplicitEnabled(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nwatermark:\n  text: BORRADOR\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil || node.Watermark == nil {
		t.Fatal("Watermark should not be nil")
	}
	if !node.Watermark.Enabled {
		t.Error("Watermark.Enabled = false, want true when the block is declared without an explicit 'enabled' key")
	}
}

// TestFrontMatterParser_WatermarkExplicitDisabled covers `enabled: false`
// explicitly overriding the implicit-true default.
func TestFrontMatterParser_WatermarkExplicitDisabled(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nwatermark:\n  enabled: false\n  text: BORRADOR\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node == nil || node.Watermark == nil {
		t.Fatal("Watermark should not be nil")
	}
	if node.Watermark.Enabled {
		t.Error("Watermark.Enabled = true, want false for an explicit 'enabled: false'")
	}
}

// TestFrontMatterParser_WatermarkFullMap covers every recognized key at
// once, verifying each converts and rotation normalizes via math.Mod.
func TestFrontMatterParser_WatermarkFullMap(t *testing.T) {
	p := &FrontMatterParser{}

	yaml := "---\nmode: flex\ntitle: Doc\nwatermark:\n" +
		"  text: BORRADOR\n" +
		"  color: \"#ff0000\"\n" +
		"  opacity: 0.2\n" +
		"  rotation: 315\n" +
		"  font_size: 48pt\n" +
		"  repeat: false\n" +
		"---\n\nContenido."
	node, _, diags := p.Parse(yaml)
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	w := node.Watermark
	if w == nil {
		t.Fatal("Watermark should not be nil")
	}
	if w.Text != "BORRADOR" {
		t.Errorf("Text = %q, want %q", w.Text, "BORRADOR")
	}
	if w.Color != "#ff0000" {
		t.Errorf("Color = %q, want %q", w.Color, "#ff0000")
	}
	if w.Opacity == nil || *w.Opacity != 0.2 {
		t.Errorf("Opacity = %v, want 0.2", w.Opacity)
	}
	// 315 degrees normalizes to -45 via math.Mod(315, 360) = 315, not -45 —
	// math.Mod keeps the sign of the dividend, so a positive input stays
	// positive. Only a negative or >360 input actually changes shape.
	if w.Rotation == nil || *w.Rotation != 315 {
		t.Errorf("Rotation = %v, want 315", w.Rotation)
	}
	if w.FontSize != "48pt" {
		t.Errorf("FontSize = %q, want %q", w.FontSize, "48pt")
	}
	if w.Repeat == nil || *w.Repeat != false {
		t.Errorf("Repeat = %v, want false", w.Repeat)
	}
}

// TestFrontMatterParser_WatermarkRotationNormalizes covers a rotation
// outside the conventional -360..360 range being reduced via math.Mod,
// same normalization convertRotation-equivalents use elsewhere.
func TestFrontMatterParser_WatermarkRotationNormalizes(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nwatermark:\n  text: X\n  rotation: 405\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node.Watermark == nil || node.Watermark.Rotation == nil {
		t.Fatal("Rotation should not be nil")
	}
	if *node.Watermark.Rotation != 45 {
		t.Errorf("Rotation = %v, want 45 (405 mod 360)", *node.Watermark.Rotation)
	}
}

// TestFrontMatterParser_WatermarkOpacityClamped covers an opacity outside
// 0.0-1.0 being clamped, with a warning — not silently dropped, not left
// to blow up downstream alpha math.
func TestFrontMatterParser_WatermarkOpacityClamped(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nwatermark:\n  text: X\n  opacity: 1.5\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node.Watermark == nil || node.Watermark.Opacity == nil || *node.Watermark.Opacity != 1.0 {
		t.Fatalf("Opacity should clamp to 1.0, got: %v", node.Watermark)
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT007" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT007 warning for an out-of-range opacity, got: %+v", diags)
	}
}

// TestFrontMatterParser_WatermarkUnrecognizedFontSizeUnit covers an
// unrecognized length unit for font_size: conserved verbatim, with a
// warning — same posture as PageMargins' unrecognized-unit handling.
func TestFrontMatterParser_WatermarkUnrecognizedFontSizeUnit(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nwatermark:\n  text: X\n  font_size: 3rem\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node.Watermark == nil || node.Watermark.FontSize != "3rem" {
		t.Fatalf("FontSize should be conserved as %q, got: %+v", "3rem", node.Watermark)
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT007" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT007 warning for an unrecognized font_size unit, got: %+v", diags)
	}
}

// TestFrontMatterParser_WatermarkUnrecognizedColorWarns covers a color
// string that doesn't parse as CSS: conserved verbatim, with a warning.
func TestFrontMatterParser_WatermarkUnrecognizedColorWarns(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nwatermark:\n  text: X\n  color: notacolor\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node.Watermark == nil || node.Watermark.Color != "notacolor" {
		t.Fatalf("Color should be conserved as %q, got: %+v", "notacolor", node.Watermark)
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT007" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT007 warning for an unrecognized color, got: %+v", diags)
	}
}

// TestFrontMatterParser_WatermarkAbsent covers the "not declared" case.
func TestFrontMatterParser_WatermarkAbsent(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, _ := p.Parse("---\nmode: flex\ntitle: Doc\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.Watermark != nil {
		t.Errorf("Watermark = %+v, want nil when not declared", node.Watermark)
	}
}

// TestFrontMatterParser_WatermarkUnknownKeyWarns covers a typo'd key
// ("opacty" instead of "opacity") surfacing as a warning instead of being
// silently dropped.
func TestFrontMatterParser_WatermarkUnknownKeyWarns(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nwatermark:\n  text: X\n  opacty: 0.5\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node.Watermark == nil || node.Watermark.Text != "X" {
		t.Fatal("Watermark.Text should still apply from the correctly-spelled 'text' key")
	}
	if node.Watermark.Opacity != nil {
		t.Errorf("Opacity = %v, want nil — 'opacty' is a typo, not 'opacity'", node.Watermark.Opacity)
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT007" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT007 warning for the unrecognized 'opacty' key, got: %+v", diags)
	}
}

// TestFrontMatterParser_WatermarkDuplicateKeyWarns covers a repeated key,
// where the last occurrence wins with a warning.
func TestFrontMatterParser_WatermarkDuplicateKeyWarns(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nwatermark:\n  text: First\n  text: Second\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node.Watermark == nil || node.Watermark.Text != "Second" {
		t.Fatalf("Text should be %q (last occurrence wins), got: %+v", "Second", node.Watermark)
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT007" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT007 warning for the duplicate 'text' key, got: %+v", diags)
	}
}

// TestFrontMatterParser_WatermarkScalarBoolRejected covers `watermark:
// false` — YAML's node.Decode into a string target is lenient and would
// otherwise stringify the bool ("false" -> Text: "false"), silently
// turning ON a watermark that reads "false" instead of surfacing a
// warning. Only a genuine string scalar counts as the shorthand.
func TestFrontMatterParser_WatermarkScalarBoolRejected(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nwatermark: false\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node.Watermark != nil {
		t.Errorf("Watermark = %+v, want nil for a bool scalar", node.Watermark)
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT007" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT007 warning for a bool 'watermark:' scalar, got: %+v", diags)
	}
}

// TestFrontMatterParser_WatermarkScalarIntRejected covers `watermark: 123`
// — same leniency hole as the bool case, with a number instead.
func TestFrontMatterParser_WatermarkScalarIntRejected(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nwatermark: 123\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node.Watermark != nil {
		t.Errorf("Watermark = %+v, want nil for an int scalar", node.Watermark)
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT007" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT007 warning for an int 'watermark:' scalar, got: %+v", diags)
	}
}

// TestFrontMatterParser_WatermarkMapTextBoolRejected covers the map-form
// equivalent, `watermark: {text: false}` — the same Decode leniency
// applies to the 'text' key inside the map, not just the top-level
// scalar shorthand.
func TestFrontMatterParser_WatermarkMapTextBoolRejected(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nwatermark:\n  text: false\n---\n\nContenido.")
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("unexpected error-severity diagnostic: %v", d)
		}
	}
	if node.Watermark == nil {
		t.Fatal("Watermark should not be nil — a bad 'text' key doesn't invalidate the whole block")
	}
	if node.Watermark.Text != "" {
		t.Errorf("Text = %q, want empty for a bool 'text:' value", node.Watermark.Text)
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT007" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT007 warning for a bool 'watermark.text', got: %+v", diags)
	}
}

// TestFrontMatterParser_WatermarkBadShapeDoesNotKillFrontMatter covers a
// sequence value for `watermark:` — must degrade to a warning, never a
// parse-killing error.
func TestFrontMatterParser_WatermarkBadShapeDoesNotKillFrontMatter(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nauthor: Someone\nwatermark:\n  - a\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil — a bad watermark: shape must not kill the front matter")
	}
	if node.Title != "Doc" || node.Author != "Someone" {
		t.Errorf("front matter fields should survive a bad watermark: shape, got Title=%q Author=%q", node.Title, node.Author)
	}
	if node.Watermark != nil {
		t.Errorf("Watermark = %+v, want nil for a bad shape", node.Watermark)
	}
	for _, d := range diags {
		if d.IsError() {
			t.Errorf("expected no error-severity diagnostics, got: %v", d)
		}
	}
	foundWarning := false
	for _, d := range diags {
		if d.RuleID == "FRONT007" {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Errorf("expected a FRONT007 warning for a sequence watermark:, got: %+v", diags)
	}
}
