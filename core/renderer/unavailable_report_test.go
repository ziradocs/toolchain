// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

type reportedUnavailable struct {
	kind   string
	line   int
	detail string
}

func (r reportedUnavailable) String() string {
	return fmt.Sprintf("%s@%d(%s)", r.kind, r.line, r.detail)
}

func collectUnavailable(ctx *RenderContext) *[]reportedUnavailable {
	var got []reportedUnavailable
	ctx.SetUnavailableReporter(func(kind string, line int, detail string) {
		got = append(got, reportedUnavailable{kind, line, detail})
	})
	return &got
}

// TestUnavailableReporter_KindLineAndSource: cada marcador de no disponible
// llega al reporter con su tipo, la línea del elemento (la del más interno
// dentro de un grid) y, para una imagen, la fuente tal como la escribió el
// autor. Lo que sí se dibujó no se reporta.
func TestUnavailableReporter_KindLineAndSource(t *testing.T) {
	ctx, root := offlineInlineCtx(t)
	ctx.MermaidMode = "offline-inline"
	ctx.MapMode = "offline-inline"
	if err := os.WriteFile(filepath.Join(root, "ok.png"), []byte("PNGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := collectUnavailable(ctx)

	at := func(line int) diagnostics.Position { return diagnostics.NewPosition(line, 1) }
	grid := ast.NewGridElement(at(20))
	grid.Columns = []ast.ColumnElement{{Elements: []ast.Element{
		ast.NewTextElement(at(22), "Dentro ![x](falta-grid.png)"),
	}}}
	elements := []ast.Element{
		ast.NewImageElement(at(3), "falta.png", "Logo"),
		ast.NewImageElement(at(5), "ok.png", "Bien"),
		ast.NewTextElement(at(7), "Texto ![p](falta-inline.png) y ![q](ok.png)"),
		// Sin fetcher en modo offline: el diagrama y el mapa quedan como
		// marcador (el mismo caso que un build sin Chromium).
		ast.NewMermaidElement(at(9), "flowchart", "flowchart LR\n A-->B"),
		ast.NewMapElement(at(14), "default"),
		grid,
	}
	for _, el := range elements {
		RenderElementToHTML(el, nil, ctx)
	}

	want := []reportedUnavailable{
		{"image", 3, "falta.png"},
		{"image", 7, "falta-inline.png"},
		{"diagram", 9, ""},
		{"map", 14, ""},
		{"image", 22, "falta-grid.png"},
	}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("reported %v, want %v", *got, want)
	}
	if ctx.renderLine != 0 {
		t.Errorf("renderLine left at %d after rendering", ctx.renderLine)
	}
}

// TestUnavailableReporter_HeaderLogo: un logo de encabezado que falta no deja
// marcador (se omite), pero se reporta, con línea 0.
func TestUnavailableReporter_HeaderLogo(t *testing.T) {
	ctx, _ := offlineInlineCtx(t)
	got := collectUnavailable(ctx)
	if html := renderHeaderLogo(&ast.LogoConfig{Source: "logo-falta.png"}, nil, ctx); html != "" {
		t.Fatalf("missing logo rendered: %q", html)
	}
	want := []reportedUnavailable{{"image", 0, "logo-falta.png"}}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("reported %v, want %v", *got, want)
	}
}

// TestUnavailableReporter_DefaultIsSilent: sin reporter, el render es el de
// siempre (el marcador sale igual) y nada falla por el campo nil.
func TestUnavailableReporter_DefaultIsSilent(t *testing.T) {
	ctx, _ := offlineInlineCtx(t)
	html := RenderElementToHTML(ast.NewImageElement(diagnostics.NewPosition(1, 1), "falta.png", "A"), nil, ctx)
	if html == "" {
		t.Fatal("empty render")
	}
	if got := RenderElementToHTML(ast.NewImageElement(diagnostics.NewPosition(1, 1), "falta.png", "A"), nil, nil); got == "" {
		t.Fatal("empty render with nil ctx")
	}
}
