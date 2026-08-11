// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer/chromium"
)

// TestResolvePDFOptions_Nil covers the "document has no opinion" case: a nil
// PageConfig must resolve to exactly chromium.DefaultPDFOptions(), untouched.
func TestResolvePDFOptions_Nil(t *testing.T) {
	logger := newTestLogger()

	got := resolvePDFOptions(nil, logger)
	want := chromium.DefaultPDFOptions()

	if got != want {
		t.Errorf("resolvePDFOptions(nil) = %+v, want %+v (DefaultPDFOptions untouched)", got, want)
	}
	if len(logger.warns) != 0 {
		t.Errorf("expected no warnings for a nil PageConfig, got: %v", logger.warns)
	}
}

// TestResolvePDFOptions_Size covers a recognized paper size overriding only
// the paper dimensions — margins/landscape/scale must stay at their
// DefaultPDFOptions() values.
func TestResolvePDFOptions_Size(t *testing.T) {
	logger := newTestLogger()

	got := resolvePDFOptions(&ast.PageConfig{Size: "Letter"}, logger)

	if got.PaperWidth != 8.5 || got.PaperHeight != 11 {
		t.Errorf("PaperWidth/PaperHeight = %v/%v, want 8.5/11 for Letter", got.PaperWidth, got.PaperHeight)
	}
	def := chromium.DefaultPDFOptions()
	if got.MarginTop != def.MarginTop || got.MarginBottom != def.MarginBottom {
		t.Errorf("margins should stay at the default when only size is declared, got %+v", got)
	}
	if len(logger.warns) != 0 {
		t.Errorf("expected no warnings for a recognized size, got: %v", logger.warns)
	}
}

// TestResolvePDFOptions_UnknownSizeWarnsAndKeepsDefault covers the
// documented fallback: an unrecognized page.size (already conserved
// verbatim and warned about once at parse time, FRONT006) must not fail
// PDF generation — it falls back to the default (A4) with its own warning.
func TestResolvePDFOptions_UnknownSizeWarnsAndKeepsDefault(t *testing.T) {
	logger := newTestLogger()

	got := resolvePDFOptions(&ast.PageConfig{Size: "Carta"}, logger)
	def := chromium.DefaultPDFOptions()

	if got.PaperWidth != def.PaperWidth || got.PaperHeight != def.PaperHeight {
		t.Errorf("expected the default A4 dimensions for an unrecognized size, got %+v", got)
	}
	if len(logger.warns) != 1 {
		t.Fatalf("expected exactly 1 warning for an unrecognized page.size, got: %v", logger.warns)
	}
}

// TestResolvePDFOptions_MarginsAllSides covers a fully-declared per-side
// margins map overriding every side.
func TestResolvePDFOptions_MarginsAllSides(t *testing.T) {
	logger := newTestLogger()

	got := resolvePDFOptions(&ast.PageConfig{
		Margins: &ast.PageMargins{Top: "1in", Right: "2in", Bottom: "3in", Left: "4in"},
	}, logger)

	if got.MarginTop != 1 || got.MarginRight != 2 || got.MarginBottom != 3 || got.MarginLeft != 4 {
		t.Errorf("margins = %+v, want top=1 right=2 bottom=3 left=4", got)
	}
	if len(logger.warns) != 0 {
		t.Errorf("expected no warnings for well-formed margins, got: %v", logger.warns)
	}
}

// TestResolvePDFOptions_MarginsPartialSideKeepsDefaultForOthers covers the
// trap called out in the plan: cdproto's margin fields are float64 WITHOUT
// omitempty, so an "unset = 0" internal representation would turn an
// undeclared side into a real zero-margin instruction. An undeclared side
// (empty string, the per-side map's shape for "not declared") must instead
// keep chromium.DefaultPDFOptions()'s 0.4in, not become 0.
func TestResolvePDFOptions_MarginsPartialSideKeepsDefaultForOthers(t *testing.T) {
	logger := newTestLogger()
	def := chromium.DefaultPDFOptions()

	got := resolvePDFOptions(&ast.PageConfig{
		Margins: &ast.PageMargins{Top: "1in"},
	}, logger)

	if got.MarginTop != 1 {
		t.Errorf("MarginTop = %v, want 1 (declared)", got.MarginTop)
	}
	if got.MarginRight != def.MarginRight || got.MarginBottom != def.MarginBottom || got.MarginLeft != def.MarginLeft {
		t.Errorf("undeclared sides should keep the 0.4in default, not become 0, got %+v", got)
	}
}

// TestResolvePDFOptions_UnrecognizedMarginUnitWarnsAndKeepsDefault mirrors
// TestResolvePDFOptions_UnknownSizeWarnsAndKeepsDefault for a margin side.
func TestResolvePDFOptions_UnrecognizedMarginUnitWarnsAndKeepsDefault(t *testing.T) {
	logger := newTestLogger()
	def := chromium.DefaultPDFOptions()

	got := resolvePDFOptions(&ast.PageConfig{
		Margins: &ast.PageMargins{Top: "5furlongs"},
	}, logger)

	if got.MarginTop != def.MarginTop {
		t.Errorf("MarginTop = %v, want the 0.4in default for an unrecognized unit", got.MarginTop)
	}
	if len(logger.warns) != 1 {
		t.Fatalf("expected exactly 1 warning for an unrecognized margin unit, got: %v", logger.warns)
	}
}

// TestResolvePDFOptions_PreservesHeaderFooterAndScaleFields covers the other
// trap called out in the plan: resolvePDFOptions must start from
// chromium.DefaultPDFOptions() and only ever override PaperWidth/
// PaperHeight/Margin* — never reset DisplayHeaderFooter/HeaderTemplate/
// FooterTemplate/Scale, which pdf.go sets separately after this call.
func TestResolvePDFOptions_PreservesScaleField(t *testing.T) {
	logger := newTestLogger()

	got := resolvePDFOptions(&ast.PageConfig{Size: "A4", Margins: &ast.PageMargins{Top: "1in"}}, logger)
	def := chromium.DefaultPDFOptions()

	if got.Scale != def.Scale {
		t.Errorf("Scale = %v, want the untouched default %v", got.Scale, def.Scale)
	}
}
