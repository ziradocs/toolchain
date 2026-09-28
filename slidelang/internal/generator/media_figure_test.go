//go:build !js

// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

func TestMediaPosterCaptionInSlideHTML(t *testing.T) {
	doc := parseSlideLang(t, "---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n  <<video src=\"https://example.com/demo.mp4\" poster=\"https://example.com/poster.jpg\" caption=\"Product demo\" controls>>\n")
	dir := t.TempDir()
	if err := New(util.NewNoop()).GenerateWithOptions(doc, "html", dir, GeneratorOptions{AssetRoot: dir}, renderer.NewDefaultRenderContext()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "deck.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	for _, want := range []string{`poster="https://example.com/poster.jpg"`, `<figure class="slidelang-media-figure">`, `<figcaption>Product demo</figcaption>`} {
		if !strings.Contains(html, want) {
			t.Fatalf("slide HTML lacks %q", want)
		}
	}
}
