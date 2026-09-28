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

func TestQuizPollResultsInSlideHTML(t *testing.T) {
	doc := parseSlideLang(t, "---\nmode: strict\nast_capabilities: [quiz-poll-results-v1]\n---\nSLIDE content\n  title: \"S\"\n  <<poll>>\n    question: \"Plan?\"\n    options: [\"Basic\", \"Pro\"]\n    results: [35, 65]\n    responses: 40\n  <<end>>\n")
	dir := t.TempDir()
	if err := New(util.NewNoop()).GenerateWithOptions(doc, "html", dir, GeneratorOptions{AssetRoot: dir}, renderer.NewDefaultRenderContext()); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "deck.html"))
	html := string(data)
	for _, want := range []string{">35%</span>", ">65%</span>", "40 responses"} {
		if !strings.Contains(html, want) {
			t.Fatalf("slide HTML lacks %q", want)
		}
	}
}
