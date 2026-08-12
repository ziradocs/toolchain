// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tocFixture mirrors numberingFixture's shape (see numbering_test.go) for
// the `toc:` defaulting logic (issue #115 follow-up, PR 3): a document that
// explicitly opts out via `toc: false`.
const tocFixture = "---\ntitle: \"Test Document\"\ntoc: false\n---\n\n# Preamble\n\nIntro text.\n\n# Real Section\n\nContent here.\n"

// buildHTML runs the build command against a fixture file with the given
// extra flags and returns the generated HTML content — used for the
// toc.depth tests below, since Markdown's TOC has no notion of depth (its
// loop only ever walks top-level ContentBlocks, see markdown.go's Generate).
func buildHTML(t *testing.T, fixture string, extraArgs ...string) string {
	t.Helper()

	tmpDir := t.TempDir()
	docPath := filepath.Join(tmpDir, "doc.doclang")
	if err := os.WriteFile(docPath, []byte(fixture), 0644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	outDir := filepath.Join(tmpDir, "out")
	args := append([]string{docPath, "--format", "html", "--output", outDir}, extraArgs...)

	cmd := NewBuildCommand(nil, nil, nil, nil, nil)
	cmd.SetArgs(args)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	if err := cmd.Execute(); err != nil {
		t.Fatalf("build failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "doc.html"))
	if err != nil {
		t.Fatalf("failed to read generated html: %v", err)
	}
	return string(data)
}

// TestBuild_FrontMatterTOCFalseSuppressesDefault covers the same opt-out gap
// #100 closed for `numbering:`, one key over: `toc: false` in front matter
// must be the default for --toc when the flag isn't explicitly passed.
func TestBuild_FrontMatterTOCFalseSuppressesDefault(t *testing.T) {
	content := buildMarkdown(t, tocFixture)

	if strings.Contains(content, "## Table of Contents") {
		t.Fatalf("expected TOC to stay disabled per front matter, got a TOC section:\n%s", content)
	}
}

// TestBuild_ExplicitTOCFlagOverridesFrontMatterFalse covers the flag > front
// matter precedence: an explicit --toc must win even when front matter says
// toc: false.
func TestBuild_ExplicitTOCFlagOverridesFrontMatterFalse(t *testing.T) {
	content := buildMarkdown(t, tocFixture, "--toc")

	if !strings.Contains(content, "## Table of Contents") {
		t.Fatalf("expected --toc to override front matter's toc: false, got:\n%s", content)
	}
}

// TestBuild_ExplicitTOCFalseOverridesFrontMatterTrue covers the other
// direction of the same precedence rule.
func TestBuild_ExplicitTOCFalseOverridesFrontMatterTrue(t *testing.T) {
	fixture := "---\ntitle: \"Test Document\"\ntoc: true\n---\n\n# Preamble\n\nIntro text.\n\n# Real Section\n\nContent here.\n"

	content := buildMarkdown(t, fixture, "--toc=false")

	if strings.Contains(content, "## Table of Contents") {
		t.Fatalf("expected --toc=false to override front matter's toc: true, got:\n%s", content)
	}
}

// TestBuild_NoFrontMatterTOCDefaultsToTrue covers the pre-existing default:
// a document that doesn't declare `toc:` at all keeps defaulting to a TOC
// when front matter is present.
func TestBuild_NoFrontMatterTOCDefaultsToTrue(t *testing.T) {
	fixture := "---\ntitle: \"Test Document\"\n---\n\n# Preamble\n\nIntro text.\n\n# Real Section\n\nContent here.\n"

	content := buildMarkdown(t, fixture)

	if !strings.Contains(content, "## Table of Contents") {
		t.Fatalf("expected TOC to default to on when front matter doesn't declare it, got:\n%s", content)
	}
}

// tocDepthFixture has a preamble with three nested subheadings (h2/h3/h4)
// for exercising toc.depth against extractSubsections
// (core/renderer/document_html.go): maxDepth is the highest heading level
// included, with the section title itself counting as level 1 — so
// depth: 2 includes only the h2, depth: 3 adds the h3, etc. (issue #123,
// fixed in core/v2.8.1; before that release the cutoff was off-by-one and
// depth: 2 also included the h3).
const tocDepthFixtureTemplate = "---\ntitle: \"Test Document\"\ntoc:\n  depth: %d\n---\n\n# Real Section\n\n## Sub One\n\nText.\n\n### Sub Two\n\nText.\n\n#### Sub Three\n\nText.\n"

// TestBuild_TOCDepthAppliesAtBuildLevel documents toc.depth's cutoff at the
// build.go wiring level (core/renderer has its own unit tests for
// extractSubsections itself): depth: 2 must include only "Sub One" (h2) in
// the rendered sidebar TOC, excluding both "Sub Two" (h3) and "Sub Three"
// (h4).
func TestBuild_TOCDepthAppliesAtBuildLevel(t *testing.T) {
	fixture := fmt.Sprintf(tocDepthFixtureTemplate, 2)

	content := buildHTML(t, fixture)

	// Each heading renders once in the document body regardless of TOC
	// depth; a second occurrence means it also made it into the sidebar
	// TOC (InteractiveViewer defaults on alongside TOC, see build.go).
	if got := strings.Count(content, "Sub One"); got != 2 {
		t.Errorf("expected depth: 2 to list the h2 subsection in the TOC (2 occurrences: body + sidebar), got %d:\n%s", got, content)
	}
	if got := strings.Count(content, "Sub Two"); got != 1 {
		t.Errorf("expected depth: 2 to exclude the h3 subsection from the TOC (1 occurrence: body only), got %d:\n%s", got, content)
	}
	if got := strings.Count(content, "Sub Three"); got != 1 {
		t.Errorf("expected depth: 2 to exclude the h4 subsection from the TOC (1 occurrence: body only), got %d:\n%s", got, content)
	}
}

// TestBuild_TOCDepthAppliesIndependentOfExplicitTOCFlag covers the specific
// hazard called out in the plan: toc.depth has no CLI flag of its own
// (--toc only talks about enabled/disabled), so its resolution must sit
// OUTSIDE the Changed("toc") guard — otherwise `doclang build --toc` would
// silently revert an explicit `toc.depth` back to the hardcoded default of
// 3, discarding it without any diagnostic.
func TestBuild_TOCDepthAppliesIndependentOfExplicitTOCFlag(t *testing.T) {
	fixture := fmt.Sprintf(tocDepthFixtureTemplate, 2)

	content := buildHTML(t, fixture, "--toc")

	if got := strings.Count(content, "Sub Two"); got != 1 {
		t.Errorf("expected toc.depth: 2 to still exclude the h3 subsection with an explicit --toc flag present, got %d occurrences:\n%s", got, content)
	}
	if got := strings.Count(content, "Sub Three"); got != 1 {
		t.Errorf("expected toc.depth: 2 to still exclude the h4 subsection (not silently reverted to the hardcoded default of 3) with an explicit --toc flag present, got %d occurrences:\n%s", got, content)
	}
}
