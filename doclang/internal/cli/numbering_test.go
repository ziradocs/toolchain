// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// numberingFixture has a preamble section (the flex parser's first `# `
// heading, block_type "title") followed by a real section, so a numbered
// build would render "1. Real Section" — the preamble itself never gets a
// number (issue #100's second bug).
const numberingFixture = "---\ntitle: \"Test Document\"\nnumbering: false\n---\n\n# Preamble\n\nIntro text.\n\n# Real Section\n\nContent here.\n"

// buildMarkdown runs the build command against a fixture file with the given
// extra flags and returns the generated Markdown content.
func buildMarkdown(t *testing.T, fixture string, extraArgs ...string) string {
	t.Helper()

	tmpDir := t.TempDir()
	docPath := filepath.Join(tmpDir, "doc.doclang")
	if err := os.WriteFile(docPath, []byte(fixture), 0644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	outDir := filepath.Join(tmpDir, "out")
	args := append([]string{docPath, "--format", "markdown", "--output", outDir}, extraArgs...)

	cmd := NewBuildCommand(nil, nil, nil, nil, nil)
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err != nil {
		t.Fatalf("build failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "doc.md"))
	if err != nil {
		t.Fatalf("failed to read generated markdown: %v", err)
	}
	return string(data)
}

// TestBuild_FrontMatterNumberingFalseSuppressesDefault covers issue #100's
// main ask: `numbering: false` in front matter must be the default for
// --numbering when the flag isn't explicitly passed, instead of the
// unconditional `true` the defaulting logic used before
// FrontMatterNode.Numbering existed.
func TestBuild_FrontMatterNumberingFalseSuppressesDefault(t *testing.T) {
	content := buildMarkdown(t, numberingFixture)

	if strings.Contains(content, "1. Real Section") {
		t.Fatalf("expected numbering to stay disabled per front matter, got a numbered heading:\n%s", content)
	}
	if !strings.Contains(content, "## Real Section") {
		t.Fatalf("expected an unnumbered heading for the real section, got:\n%s", content)
	}
}

// TestBuild_ExplicitNumberingFlagOverridesFrontMatterFalse covers the flag >
// front matter precedence from issue #100: an explicit --numbering must win
// even when front matter says numbering: false.
func TestBuild_ExplicitNumberingFlagOverridesFrontMatterFalse(t *testing.T) {
	content := buildMarkdown(t, numberingFixture, "--numbering")

	if !strings.Contains(content, "1. Real Section") {
		t.Fatalf("expected --numbering to override front matter's numbering: false, got:\n%s", content)
	}
}

// TestBuild_ExplicitNumberingFalseOverridesFrontMatterTrue covers the other
// direction of the same precedence rule: an explicit --numbering=false must
// win even when front matter says numbering: true.
func TestBuild_ExplicitNumberingFalseOverridesFrontMatterTrue(t *testing.T) {
	fixture := "---\ntitle: \"Test Document\"\nnumbering: true\n---\n\n# Preamble\n\nIntro text.\n\n# Real Section\n\nContent here.\n"

	content := buildMarkdown(t, fixture, "--numbering=false")

	if strings.Contains(content, "1. Real Section") {
		t.Fatalf("expected --numbering=false to override front matter's numbering: true, got:\n%s", content)
	}
}

// TestBuild_NoFrontMatterNumberingDefaultsToTrue covers the pre-existing
// default: a document that doesn't declare `numbering:` at all keeps
// defaulting to numbered sections when front matter is present, same as
// before FrontMatterNode.Numbering existed.
func TestBuild_NoFrontMatterNumberingDefaultsToTrue(t *testing.T) {
	fixture := "---\ntitle: \"Test Document\"\n---\n\n# Preamble\n\nIntro text.\n\n# Real Section\n\nContent here.\n"

	content := buildMarkdown(t, fixture)

	if !strings.Contains(content, "1. Real Section") {
		t.Fatalf("expected numbering to default to on when front matter doesn't declare it, got:\n%s", content)
	}
}
