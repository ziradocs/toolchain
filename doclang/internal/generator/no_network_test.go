// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/renderer"
)

func plantUMLDoc(t *testing.T) *ast.AST {
	t.Helper()
	doc := parseInline(t, "---\ntitle: P\nlang: es\nmode: flex\n---\n\n# P\n\nTexto.\n")
	block := &doc.ContentBlocks[len(doc.ContentBlocks)-1]
	block.Elements = append(block.Elements, ast.NewPlantUMLElement(diagnostics.NewPosition(9, 1), "sequence", "@startuml\nA -> B\n@enduml"))
	return doc
}

// TestDOCX_PlantUMLUsesConfiguredServer (issue #472): el DOCX le pide el
// PlantUML al mismo servidor que el HTML y el PDF: --plantuml-server o, con
// --diagram-backend=kroki, --kroki-server. Antes tenía fijo
// www.plantuml.com.
func TestDOCX_PlantUMLUsesConfiguredServer(t *testing.T) {
	cases := []struct {
		name       string
		opts       func(url string) GeneratorOptions
		wantPrefix string
	}{
		{"plantuml-server", func(u string) GeneratorOptions {
			return GeneratorOptions{Format: "docx", PlantUMLServer: u + "/plantuml"}
		}, "/plantuml/png/"},
		{"kroki", func(u string) GeneratorOptions {
			return GeneratorOptions{Format: "docx", DiagramBackend: "kroki", KrokiServer: u}
		}, "/plantuml/png"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv, hits, lastPath := pngServer(t, 120, 60)
			out := filepath.Join(t.TempDir(), "out.docx")
			if err := New(newTestLogger()).Generate(plantUMLDoc(t), out, c.opts(srv.URL)); err != nil {
				t.Fatal(err)
			}
			if hits.Load() != 1 {
				t.Fatalf("server got %d requests, want 1", hits.Load())
			}
			if p, _ := lastPath.Load().(string); !strings.HasPrefix(p, c.wantPrefix) {
				t.Errorf("request path %q, want prefix %q", p, c.wantPrefix)
			}
			if got := docxDescrs(t, out, "word/document.xml"); len(got) != 1 {
				t.Errorf("want the diagram image in the DOCX, got %d images", len(got))
			}
		})
	}
}

// TestDOCX_NoNetwork (issue #472): con NoNetwork el DOCX no le pide nada al
// servidor de PlantUML ni al de tiles, aunque estén configurados, y los
// deja como marcador con un aviso.
func TestDOCX_NoNetwork(t *testing.T) {
	diagrams, diagramHits, _ := pngServer(t, 120, 60)
	tiles, tileHits, _ := pngServer(t, 256, 256)
	doc := plantUMLDoc(t)
	m := ast.NewMapElement(diagnostics.NewPosition(11, 1), "default")
	m.Zoom = 4
	block := &doc.ContentBlocks[len(doc.ContentBlocks)-1]
	block.Elements = append(block.Elements, m)

	opts := GeneratorOptions{Format: "docx", PlantUMLServer: diagrams.URL, NoNetwork: true}
	if _, ok := any(renderer.MapConfig{}).(mapTileServerConfig); ok {
		opts.TileServer = tiles.URL + "/{z}/{x}/{y}.png"
	}
	out := filepath.Join(t.TempDir(), "out.docx")
	logger := newTestLogger()
	if err := New(logger).Generate(doc, out, opts); err != nil {
		t.Fatal(err)
	}
	if diagramHits.Load() != 0 || tileHits.Load() != 0 {
		t.Fatalf("requests with NoNetwork: diagram server %d, tile server %d", diagramHits.Load(), tileHits.Load())
	}
	if got := docxDescrs(t, out, "word/document.xml"); len(got) != 0 {
		t.Errorf("want no images, got %d", len(got))
	}
	body := docxDocumentXML(t, out)
	for _, note := range []string{"Diagrama no disponible", "Mapa no disponible"} {
		if !strings.Contains(body, note) {
			t.Errorf("DOCX lacks %q", note)
		}
	}
	warned := 0
	for _, w := range logger.warns {
		if strings.Contains(w, "--no-network") {
			warned++
		}
	}
	if warned != 2 {
		t.Errorf("want 2 --no-network warnings, got %q", logger.warns)
	}
}

// TestHTML_NoNetwork (issue #472): en el HTML offline-assets, NoNetwork deja
// el PlantUML como marcador sin pedírselo al servidor; sin la opción, el
// mismo build sí lo pide.
func TestHTML_NoNetwork(t *testing.T) {
	for _, noNet := range []bool{false, true} {
		srv, hits, _ := pngServer(t, 120, 60)
		out := filepath.Join(t.TempDir(), "out.html")
		opts := GeneratorOptions{
			Format: "html", PlantUMLMode: "offline-assets", PlantUMLFormat: "png",
			PlantUMLServer: srv.URL, NoNetwork: noNet,
		}
		if err := New(newTestLogger()).Generate(plantUMLDoc(t), out, opts); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		marker := strings.Contains(string(data), `class="plantuml-error"`)
		if noNet && (hits.Load() != 0 || !marker) {
			t.Errorf("NoNetwork: %d requests, marker %v", hits.Load(), marker)
		}
		if !noNet && hits.Load() == 0 {
			t.Error("control build without NoNetwork never contacted the server")
		}
	}
}

// TestHTML_TileServer (Chromium real): el mapa del HTML offline-inline (la
// misma ruta que el PDF) pide los tiles a --tile-server con su User-Agent.
func TestHTML_TileServer(t *testing.T) {
	if testing.Short() {
		t.Skip("needs Chromium")
	}
	if _, ok := any(renderer.NewDefaultRenderContext()).(mapTileServerSetter); !ok {
		t.Skip("this core has no RenderContext.SetMapTileServer")
	}
	tiles, hits, lastPath := pngServer(t, 256, 256)
	doc := parseInline(t, "---\ntitle: T\nmode: flex\n---\n\n# T\n\nTexto.\n")
	m := ast.NewMapElement(diagnostics.NewPosition(9, 1), "default")
	m.Zoom = 4
	block := &doc.ContentBlocks[len(doc.ContentBlocks)-1]
	block.Elements = append(block.Elements, m)
	out := filepath.Join(t.TempDir(), "out.html")
	opts := GeneratorOptions{Format: "html", MapMode: "offline-inline", TileServer: tiles.URL + "/t/{z}/{x}/{y}.png"}
	if err := New(newTestLogger()).Generate(doc, out, opts); err != nil {
		if strings.Contains(err.Error(), "hromium") {
			t.Skipf("no Chromium: %v", err)
		}
		t.Fatal(err)
	}
	if hits.Load() == 0 {
		t.Fatal("tile server never contacted")
	}
	if p, _ := lastPath.Load().(string); !strings.HasPrefix(p, "/t/") {
		t.Errorf("tile request path %q", p)
	}
	data, err := os.ReadFile(out)
	if err != nil || !strings.Contains(string(data), `class="map-image map-inline"`) {
		t.Fatalf("map not inlined (err %v)", err)
	}
}
