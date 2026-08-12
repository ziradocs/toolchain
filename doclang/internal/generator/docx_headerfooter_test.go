// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"testing"

	docx "github.com/mmonterroca/docxgo/v2"
	"github.com/mmonterroca/docxgo/v2/domain"
	"go.ziradocs.com/core/v2/ast"
)

// Estos tests cubren issue #117 para el backend DOCX: opts.HeaderFooter
// nunca se consumía, así que un documento con `header:`/`footer:` válido
// producía un .docx sin ningún header/footer, pese a que docxgo ya tenía
// soporte nativo (Section.Header/Footer + AddField) desde antes.

func newTestDocxSection(t *testing.T) domain.Section {
	t.Helper()
	doc := docx.NewDocument()
	section, err := doc.DefaultSection()
	if err != nil {
		t.Fatalf("DefaultSection failed: %v", err)
	}
	return section
}

func TestRenderHeaderFooter_NilConfig_WritesNothing(t *testing.T) {
	doc := docx.NewDocument()

	if err := renderHeaderFooterOnDoc(doc, nil, nil); err != nil {
		t.Fatalf("renderHeaderFooter failed: %v", err)
	}

	section, err := doc.DefaultSection()
	if err != nil {
		t.Fatalf("DefaultSection failed: %v", err)
	}
	header, err := section.Header(domain.HeaderDefault)
	if err != nil {
		t.Fatalf("Header failed: %v", err)
	}
	if len(header.Paragraphs()) != 0 {
		t.Errorf("expected no header paragraphs for nil config, got %d", len(header.Paragraphs()))
	}
}

func TestRenderHeaderFooter_DisabledDoesNotWrite(t *testing.T) {
	doc := docx.NewDocument()
	hf := &ast.HeaderFooterConfig{
		Header: &ast.HeaderConfig{Enabled: false, Text: &ast.HeaderFooterText{Center: "should not appear"}},
	}

	if err := renderHeaderFooterOnDoc(doc, hf, nil); err != nil {
		t.Fatalf("renderHeaderFooter failed: %v", err)
	}

	section, _ := doc.DefaultSection()
	header, _ := section.Header(domain.HeaderDefault)
	if len(header.Paragraphs()) != 0 {
		t.Errorf("expected enabled:false to write nothing, got %d paragraphs", len(header.Paragraphs()))
	}
}

func TestRenderHeaderFooterZones_WritesOneAlignedParagraphPerZone(t *testing.T) {
	section := newTestDocxSection(t)
	header, err := section.Header(domain.HeaderDefault)
	if err != nil {
		t.Fatalf("Header failed: %v", err)
	}

	text := &ast.HeaderFooterText{Left: "Izquierda", Center: "Centro", Right: "Derecha"}
	if err := renderHeaderFooterZones(header, text, nil); err != nil {
		t.Fatalf("renderHeaderFooterZones failed: %v", err)
	}

	paras := header.Paragraphs()
	if len(paras) != 3 {
		t.Fatalf("expected 3 paragraphs (one per non-empty zone), got %d", len(paras))
	}

	wantAlign := []domain.Alignment{domain.AlignmentLeft, domain.AlignmentCenter, domain.AlignmentRight}
	wantText := []string{"Izquierda", "Centro", "Derecha"}
	for i, p := range paras {
		if p.Alignment() != wantAlign[i] {
			t.Errorf("paragraph %d: alignment = %v, want %v", i, p.Alignment(), wantAlign[i])
		}
		if p.Text() != wantText[i] {
			t.Errorf("paragraph %d: text = %q, want %q", i, p.Text(), wantText[i])
		}
	}
}

func TestRenderHeaderFooterZones_SkipsEmptyZones(t *testing.T) {
	section := newTestDocxSection(t)
	header, _ := section.Header(domain.HeaderDefault)

	text := &ast.HeaderFooterText{Center: "Solo centro"}
	if err := renderHeaderFooterZones(header, text, nil); err != nil {
		t.Fatalf("renderHeaderFooterZones failed: %v", err)
	}

	paras := header.Paragraphs()
	if len(paras) != 1 {
		t.Fatalf("expected 1 paragraph (only Center is non-empty), got %d", len(paras))
	}
	if paras[0].Text() != "Solo centro" {
		t.Errorf("text = %q, want %q", paras[0].Text(), "Solo centro")
	}
}

func TestRenderHeaderFooterZones_ProcessesVariables(t *testing.T) {
	section := newTestDocxSection(t)
	header, _ := section.Header(domain.HeaderDefault)

	text := &ast.HeaderFooterText{Center: "{{title}} — confidencial"}
	variables := map[string]interface{}{"title": "Reporte Q3"}
	if err := renderHeaderFooterZones(header, text, variables); err != nil {
		t.Fatalf("renderHeaderFooterZones failed: %v", err)
	}

	paras := header.Paragraphs()
	if len(paras) != 1 || paras[0].Text() != "Reporte Q3 — confidencial" {
		t.Fatalf("expected variable substitution, got paragraphs: %v", paras)
	}
}

// TestRenderFooterPageNumbers_InsertsNativeWordFields verifica que
// {{current}}/{{total}} se traducen a campos NATIVOS de Word
// (FieldTypePageNumber/FieldTypeNumPages) — no a texto estático — para que
// Word muestre la paginación real, no un conteo de ContentBlocks.
func TestRenderFooterPageNumbers_InsertsNativeWordFields(t *testing.T) {
	section := newTestDocxSection(t)
	footer, err := section.Footer(domain.FooterDefault)
	if err != nil {
		t.Fatalf("Footer failed: %v", err)
	}

	config := &ast.PageNumbersConfig{
		Enabled:  true,
		Format:   "Página {{current}} de {{total}}",
		Position: "center",
	}
	if err := renderFooterPageNumbers(footer, config, nil); err != nil {
		t.Fatalf("renderFooterPageNumbers failed: %v", err)
	}

	paras := footer.Paragraphs()
	if len(paras) != 1 {
		t.Fatalf("expected exactly 1 footer paragraph, got %d", len(paras))
	}
	if paras[0].Alignment() != domain.AlignmentCenter {
		t.Errorf("alignment = %v, want AlignmentCenter", paras[0].Alignment())
	}

	runs := paras[0].Runs()
	if len(runs) != 4 {
		t.Fatalf("expected 4 runs (\"Página \", <field>, \" de \", <field>), got %d", len(runs))
	}
	if runs[0].Text() != "Página " {
		t.Errorf("run[0].Text() = %q, want %q", runs[0].Text(), "Página ")
	}
	if fields := runs[1].Fields(); len(fields) != 1 || fields[0].Type() != domain.FieldTypePageNumber {
		t.Errorf("run[1] expected a single FieldTypePageNumber field, got %v", fields)
	}
	if runs[2].Text() != " de " {
		t.Errorf("run[2].Text() = %q, want %q", runs[2].Text(), " de ")
	}
	if fields := runs[3].Fields(); len(fields) != 1 || fields[0].Type() != domain.FieldTypePageCount {
		t.Errorf("run[3] expected a single FieldTypePageCount field, got %v", fields)
	}
}

func TestRenderFooterPageNumbers_DefaultFormat(t *testing.T) {
	section := newTestDocxSection(t)
	footer, _ := section.Footer(domain.FooterDefault)

	if err := renderFooterPageNumbers(footer, &ast.PageNumbersConfig{Enabled: true}, nil); err != nil {
		t.Fatalf("renderFooterPageNumbers failed: %v", err)
	}

	runs := footer.Paragraphs()[0].Runs()
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs (<field>, \" / \", <field>) for the default format, got %d", len(runs))
	}
	if runs[1].Text() != " / " {
		t.Errorf("runs[1].Text() = %q, want %q", runs[1].Text(), " / ")
	}
}

// TestRenderFooterPageNumbers_SubstitutesDocumentVariables cubre un
// hallazgo de code review sobre este PR: Format es texto de front matter
// como cualquier otro (mismo mecanismo que header.text/footer.text), así
// que un placeholder de documento como {{company}} debe resolverse — antes
// de este fix, PDF y DOCX ignoraban por completo el mapa de variables acá
// y solo HTML/slidelang sustituían {{company}}.
func TestRenderFooterPageNumbers_SubstitutesDocumentVariables(t *testing.T) {
	section := newTestDocxSection(t)
	footer, _ := section.Footer(domain.FooterDefault)

	config := &ast.PageNumbersConfig{
		Enabled: true,
		Format:  "{{company}} — {{current}} / {{total}}",
	}
	variables := map[string]interface{}{"company": "Acme"}
	if err := renderFooterPageNumbers(footer, config, variables); err != nil {
		t.Fatalf("renderFooterPageNumbers failed: %v", err)
	}

	runs := footer.Paragraphs()[0].Runs()
	if len(runs) != 4 {
		t.Fatalf("expected 4 runs (\"Acme — \", <field>, \" / \", <field>), got %d: %v", len(runs), runs)
	}
	if runs[0].Text() != "Acme — " {
		t.Errorf("runs[0].Text() = %q, want %q (document variable must be substituted)", runs[0].Text(), "Acme — ")
	}
}

func TestSplitPageNumberFormat(t *testing.T) {
	tests := []struct {
		name   string
		format string
		want   []pageNumberPart
	}{
		{
			name:   "current and total with separator",
			format: "{{current}} / {{total}}",
			want: []pageNumberPart{
				{kind: pageNumberPartCurrent},
				{kind: pageNumberPartText, text: " / "},
				{kind: pageNumberPartTotal},
			},
		},
		{
			name:   "only current, with prefix",
			format: "Page {{current}}",
			want: []pageNumberPart{
				{kind: pageNumberPartText, text: "Page "},
				{kind: pageNumberPartCurrent},
			},
		},
		{
			name:   "no tokens at all",
			format: "static text",
			want:   []pageNumberPart{{kind: pageNumberPartText, text: "static text"}},
		},
		{
			name:   "tokens adjacent with no separator",
			format: "{{current}}{{total}}",
			want: []pageNumberPart{
				{kind: pageNumberPartCurrent},
				{kind: pageNumberPartTotal},
			},
		},
		{
			name:   "total before current",
			format: "of {{total}}, page {{current}}",
			want: []pageNumberPart{
				{kind: pageNumberPartText, text: "of "},
				{kind: pageNumberPartTotal},
				{kind: pageNumberPartText, text: ", page "},
				{kind: pageNumberPartCurrent},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitPageNumberFormat(tt.format)
			if len(got) != len(tt.want) {
				t.Fatalf("splitPageNumberFormat(%q) = %v, want %v", tt.format, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("part %d = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// renderHeaderFooterOnDoc es un pequeño wrapper para poder llamar
// (*DOCXGenerator).renderHeaderFooter sin construir un DOCXGenerator
// completo — el método no toca ningún otro campo del receiver.
func renderHeaderFooterOnDoc(doc domain.Document, hf *ast.HeaderFooterConfig, variables map[string]interface{}) error {
	g := &DOCXGenerator{}
	return g.renderHeaderFooter(doc, hf, variables)
}
