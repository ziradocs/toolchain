// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/util"
)

// playgroundTOCFixture es el documento que comparte la prueba del TOC de la
// vista previa del playground (slidelang/internal/docpreview,
// TestRenderTOCMatchesBuild): las dos comparan contra la misma lista.
const playgroundTOCFixture = "../../../slidelang/internal/docpreview/testdata/toc_depth_2"

// sidebarTOCEntries lee el TOC del viewer interactivo (<nav class="sidebar-toc">)
// que doclang build escribe como líneas "nivel href texto", sin el número de
// sección: nivel 1 para un toc-link y 2 para un toc-sub-link. El sidebar no
// distingue niveles más hondos (un ### también es toc-sub-link), por eso el
// documento compartido usa toc.depth: 2.
func sidebarTOCEntries(page string) string {
	nav := regexp.MustCompile(`<nav class="sidebar-toc"[\s\S]*?</nav>`).FindString(page)
	var b strings.Builder
	link := regexp.MustCompile(`<a href="(#[^"]*)" class="toc-link( toc-sub-link)?"[^>]*>\s*(?:<span class="toc-number">[^<]*</span>\s*)?<span class="toc-text">([^<]*)</span>`)
	for _, m := range link.FindAllStringSubmatch(nav, -1) {
		level := 1
		if m[2] != "" {
			level = 2
		}
		fmt.Fprintf(&b, "%d %s %s\n", level, m[1], html.UnescapeString(m[3]))
	}
	return b.String()
}

// doclang build con toc.depth: 2 lista las secciones y sus ##, sin el ###, y
// es la lista que la vista previa del playground tiene que dar.
func TestBuildTOCMatchesPlaygroundGolden(t *testing.T) {
	src, err := os.ReadFile(playgroundTOCFixture + ".doclang")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(playgroundTOCFixture + ".toc.txt")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	log := util.NewNoop()
	prepared, err := Prepare(log, src, PrepareOptions{MaxInputBytes: 1 << 20, IncludeRoot: dir})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	out := filepath.Join(dir, "toc.html")
	if err := Generate(log, prepared, "toc_depth_2.doclang", GenerateOptions{Format: "html", OutputFile: out, RenderMode: "browser", DiagramBackend: "chromium"}); err != nil {
		t.Fatalf("Generate: %v", err)
	}
	page, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if got := sidebarTOCEntries(string(page)); got != string(want) {
		t.Errorf("doclang build TOC:\n%s\nwant (playground golden):\n%s", got, want)
	}
}
