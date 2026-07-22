// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package xref

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func pos() diagnostics.Position { return diagnostics.Position{Line: 1, Column: 1} }

func TestAssignNumbers_SeparateCountersPerKind(t *testing.T) {
	img1 := ast.NewImageElement(pos(), "a.png", "a")
	img1.Label = "fig:uno"
	img2 := ast.NewImageElement(pos(), "b.png", "b")
	img2.Label = "fig:dos"
	tbl1 := ast.NewTableElement(pos())
	tbl1.Label = "tbl:uno"
	unlabeled := ast.NewImageElement(pos(), "c.png", "c") // sin label: no se numera

	block := ast.NewContentBlock(pos(), "content")
	block.Elements = append(block.Elements, img1, tbl1, img2, unlabeled)
	doc := ast.NewAST(pos())
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	table, err := AssignNumbers(doc)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}

	if img1.Number != 1 {
		t.Errorf("img1.Number = %d, want 1", img1.Number)
	}
	if img2.Number != 2 {
		t.Errorf("img2.Number = %d, want 2 (figuras cuentan independiente de tablas)", img2.Number)
	}
	if tbl1.Number != 1 {
		t.Errorf("tbl1.Number = %d, want 1 (tablas tienen su propio contador)", tbl1.Number)
	}
	if unlabeled.Number != 0 {
		t.Errorf("unlabeled.Number = %d, want 0 (sin label no se numera)", unlabeled.Number)
	}

	if table["fig:uno"].Kind != KindFigure || table["fig:uno"].Number != 1 {
		t.Errorf("table[fig:uno] = %+v, want {Figura 1 ...}", table["fig:uno"])
	}
	if table["tbl:uno"].Kind != KindTable || table["tbl:uno"].Number != 1 {
		t.Errorf("table[tbl:uno] = %+v, want {Tabla 1 ...}", table["tbl:uno"])
	}
}

func TestAssignNumbers_DuplicateLabelErrors(t *testing.T) {
	img1 := ast.NewImageElement(pos(), "a.png", "a")
	img1.Label = "fig:x"
	img2 := ast.NewImageElement(pos(), "b.png", "b")
	img2.Label = "fig:x" // duplicado

	block := ast.NewContentBlock(pos(), "content")
	block.Elements = append(block.Elements, img1, img2)
	doc := ast.NewAST(pos())
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	_, err := AssignNumbers(doc)
	if err == nil {
		t.Fatal("esperaba error por label duplicado")
	}
}

func TestResolveRefs_InTextElement(t *testing.T) {
	table := Table{"fig:x": {Kind: KindFigure, Number: 3, AnchorID: "fig-x"}}
	text := ast.NewTextElement(pos(), `ver \ref{fig:x} para más detalle`)
	block := ast.NewContentBlock(pos(), "content")
	block.Elements = append(block.Elements, text)
	doc := ast.NewAST(pos())
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	if err := ResolveRefs(doc, table); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	want := "ver [Figura 3](#fig-x) para más detalle"
	if text.Content != want {
		t.Errorf("Content = %q, want %q", text.Content, want)
	}
}

func TestResolveRefs_InsideBulletPoint(t *testing.T) {
	// Caso señalado explícitamente: un \ref dentro de un item de lista debe
	// resolverse igual que en TextElement — NO solo en TextElement.Content.
	table := Table{"tbl:datos": {Kind: KindTable, Number: 2, AnchorID: "tbl-datos"}}
	points := ast.NewPointsElement(pos())
	item := ast.NewPointItem(pos(), `resumen en \ref{tbl:datos}`)
	points.Items = append(points.Items, *item)
	block := ast.NewContentBlock(pos(), "content")
	block.Elements = append(block.Elements, points)
	doc := ast.NewAST(pos())
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	if err := ResolveRefs(doc, table); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	want := "resumen en [Tabla 2](#tbl-datos)"
	if got := doc.ContentBlocks[0].Elements[0].(*ast.PointsElement).Items[0].Content; got != want {
		t.Errorf("PointItem.Content = %q, want %q", got, want)
	}
}

func TestResolveRefs_InsideCaption(t *testing.T) {
	// Caso señalado explícitamente: un \ref dentro de un caption (no solo
	// TextElement).
	table := Table{"fig:otra": {Kind: KindFigure, Number: 5, AnchorID: "fig-otra"}}
	img := ast.NewImageElement(pos(), "x.png", "alt")
	img.Caption = `comparar con \ref{fig:otra}`
	block := ast.NewContentBlock(pos(), "content")
	block.Elements = append(block.Elements, img)
	doc := ast.NewAST(pos())
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	if err := ResolveRefs(doc, table); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	want := "comparar con [Figura 5](#fig-otra)"
	if img.Caption != want {
		t.Errorf("Caption = %q, want %q", img.Caption, want)
	}
}

func TestResolveRefs_UnresolvedLabelErrors(t *testing.T) {
	text := ast.NewTextElement(pos(), `ver \ref{no-existe}`)
	block := ast.NewContentBlock(pos(), "content")
	block.Elements = append(block.Elements, text)
	doc := ast.NewAST(pos())
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	err := ResolveRefs(doc, Table{})
	if err == nil {
		t.Fatal("esperaba error por \\ref sin resolver")
	}
	if !strings.Contains(err.Error(), "no-existe") {
		t.Errorf("el error debe nombrar el label sin resolver: %v", err)
	}
}

func TestTransform_ForwardReference(t *testing.T) {
	// \ref{fig:despues} PRECEDE a la definición de "fig:despues" en el
	// documento — solo funciona porque AssignNumbers completa TODO el
	// documento antes de que ResolveRefs corra (dos fases, no interleaved).
	textBefore := ast.NewTextElement(pos(), `como se ve en \ref{fig:despues}`)
	img := ast.NewImageElement(pos(), "x.png", "alt")
	img.Label = "fig:despues"

	block := ast.NewContentBlock(pos(), "content")
	block.Elements = append(block.Elements, textBefore, img)
	doc := ast.NewAST(pos())
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	out, err := Transform(doc)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	got := out.ContentBlocks[0].Elements[0].(*ast.TextElement).Content
	want := "como se ve en [Figura 1](#fig-despues)"
	if got != want {
		t.Errorf("Content = %q, want %q (forward reference no resuelto)", got, want)
	}
}

func TestTransform_NoLabelsIsNoOp(t *testing.T) {
	text := ast.NewTextElement(pos(), "texto normal sin refs")
	block := ast.NewContentBlock(pos(), "content")
	block.Elements = append(block.Elements, text)
	doc := ast.NewAST(pos())
	doc.ContentBlocks = append(doc.ContentBlocks, *block)

	out, err := Transform(doc)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if out.ContentBlocks[0].Elements[0].(*ast.TextElement).Content != "texto normal sin refs" {
		t.Error("documento sin labels/refs no debería modificarse")
	}
}

func TestAnchorID_Slugifies(t *testing.T) {
	cases := map[string]string{
		"fig:arquitectura":  "fig-arquitectura",
		"Fig: Arquitectura": "fig-arquitectura",
		"  espacios  ":      "espacios",
		"a---b":             "a-b",
	}
	for in, want := range cases {
		if got := AnchorID(in); got != want {
			t.Errorf("AnchorID(%q) = %q, want %q", in, got, want)
		}
	}
}
