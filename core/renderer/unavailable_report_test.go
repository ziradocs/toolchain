// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

// TestUnavailableReporter_OfflineAssetsKeepsMissing: HTML offline-assets
// deja el src relativo de una imagen local aunque el archivo no exista; el
// HTML no cambia, pero se reporta, una vez por fuente (un grid vuelve a
// resolver las imágenes de sus hijos).
func TestUnavailableReporter_OfflineAssetsKeepsMissing(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.png"), []byte("PNG"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := NewDefaultRenderContext()
	ctx.ImageMode = "offline-assets"
	ctx.AssetRoot = root
	got := collectUnavailable(ctx)
	at := func(line int) diagnostics.Position { return diagnostics.NewPosition(line, 1) }
	grid := ast.NewGridElement(at(20))
	grid.Columns = []ast.ColumnElement{{Elements: []ast.Element{ast.NewTextElement(at(21), "x ![g](falta-grid.png)")}}}
	html := ""
	for _, el := range []ast.Element{
		ast.NewImageElement(at(3), "falta.png", "A"),
		ast.NewImageElement(at(4), "ok.png", "B"),
		ast.NewTextElement(at(5), "t ![i](falta-inline.png) ![j](ok.png)"),
		grid,
	} {
		html += RenderElementToHTML(el, nil, ctx)
	}
	if !strings.Contains(html, `src="falta.png"`) {
		t.Errorf("the offline-assets HTML changed: %s", html)
	}
	want := []reportedUnavailable{{"image", 3, "falta.png"}, {"image", 5, "falta-inline.png"}, {"image", 21, "falta-grid.png"}}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("reported %v, want %v", *got, want)
	}
}

// TestUnavailableReporter_Media: un video sin fuente, con una fuente
// bloqueada o local que no se pudo inlinear en offline-inline (el PDF) se
// reporta como medio no disponible.
func TestUnavailableReporter_Media(t *testing.T) {
	ctx, _ := offlineInlineCtx(t)
	got := collectUnavailable(ctx)
	at := func(line int) diagnostics.Position { return diagnostics.NewPosition(line, 1) }
	video := func(line int, src string) *ast.MediaElement {
		m := ast.NewMediaElement(at(line), "video", src)
		return m
	}
	for _, el := range []ast.Element{video(3, ""), video(4, "javascript:alert(1)"), video(5, "no-existe.mp4")} {
		RenderElementToHTML(el, nil, ctx)
	}
	want := []reportedUnavailable{{"media", 3, ""}, {"media", 4, "javascript:alert(1)"}, {"media", 5, "no-existe.mp4"}}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("reported %v, want %v", *got, want)
	}
}

// TestLocalAsset_QueryString: "pic.png?v=2" es el archivo pic.png, como en el
// navegador: en offline-inline se inlinea y en offline-assets no se reporta
// como faltante. Antes se buscaba el archivo "pic.png?v=2".
func TestLocalAsset_QueryString(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "img"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "img", "pic.png"), []byte("PNGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"offline-inline", "offline-assets"} {
		ctx := NewDefaultRenderContext()
		ctx.ImageMode = mode
		ctx.AssetRoot = root
		got := collectUnavailable(ctx)
		pos := diagnostics.NewPosition(1, 1)
		html := RenderElementToHTML(ast.NewImageElement(pos, "img/pic.png?v=2", "P"), nil, ctx) +
			RenderElementToHTML(ast.NewTextElement(pos, "t ![q](img/pic.png?v=2#x)"), nil, ctx)
		if len(*got) != 0 {
			t.Errorf("%s: reported %v for an existing file with a query", mode, *got)
		}
		if mode == "offline-inline" && strings.Count(html, "data:image/png;base64,") != 2 {
			t.Errorf("offline-inline did not inline both images: %s", html)
		}
	}
}

// TestUnavailableReporter_HeaderLogoOfflineAssets: en HTML offline-assets
// el logo que falta queda con su src relativo, como una imagen, y se
// reporta igual.
func TestUnavailableReporter_HeaderLogoOfflineAssets(t *testing.T) {
	ctx := NewDefaultRenderContext()
	ctx.ImageMode = "offline-assets"
	ctx.AssetRoot = t.TempDir()
	got := collectUnavailable(ctx)
	if html := renderHeaderLogo(&ast.LogoConfig{Source: "logo-falta.png"}, nil, ctx); !strings.Contains(html, "logo-falta.png") {
		t.Fatalf("offline-assets logo changed: %q", html)
	}
	if want := []reportedUnavailable{{"image", 0, "logo-falta.png"}}; !reflect.DeepEqual(*got, want) {
		t.Fatalf("reported %v, want %v", *got, want)
	}
}
