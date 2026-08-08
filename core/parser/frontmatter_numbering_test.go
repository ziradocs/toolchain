// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import "testing"

// TestFrontMatterParser_NumberingFalse cubre el prerrequisito de issue #100:
// `numbering: false` en el frontmatter debe llegar a
// FrontMatterNode.Numbering como *bool explícito, no perderse en el parseo.
func TestFrontMatterParser_NumberingFalse(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nnumbering: false\n---\n\nContenido.")
	for _, d := range diags {
		t.Logf("diagnostic: %v", d)
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.Numbering == nil {
		t.Fatal("Numbering = nil, want a non-nil pointer to false")
	}
	if *node.Numbering != false {
		t.Errorf("Numbering = %v, want false", *node.Numbering)
	}
}

// TestFrontMatterParser_NumberingTrue is the symmetric case: an explicit
// `numbering: true` must also round-trip as a non-nil pointer, not just the
// zero-value-collision-prone `false` case.
func TestFrontMatterParser_NumberingTrue(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, _ := p.Parse("---\nmode: flex\ntitle: Doc\nnumbering: true\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.Numbering == nil {
		t.Fatal("Numbering = nil, want a non-nil pointer to true")
	}
	if *node.Numbering != true {
		t.Errorf("Numbering = %v, want true", *node.Numbering)
	}
}

// TestFrontMatterParser_NumberingAbsent covers the tri-state's third value:
// a document that never declares `numbering:` must leave Numbering nil, not
// default it to some concrete value inside the parser — the CLI defaulting
// logic (doclang/internal/cli/build.go) is what decides what "unset" means.
func TestFrontMatterParser_NumberingAbsent(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, _ := p.Parse("---\nmode: flex\ntitle: Doc\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.Numbering != nil {
		t.Errorf("Numbering = %v, want nil when not declared", *node.Numbering)
	}
}

// TestFrontMatterParser_NumberingLegacyMapEnabledTrue covers issue #100's
// review finding #1: `doclang init`'s `technical` and `report` templates
// emit `numbering:` as a map (`numbering:\n  enabled: true\n  style:
// 1.1.1`), which predates the tri-state bool field. That legacy shape must
// still parse — a `doclang init --template technical && doclang build`
// round-trip must not start failing with "cannot unmarshal !!map into
// bool".
func TestFrontMatterParser_NumberingLegacyMapEnabledTrue(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, diags := p.Parse("---\nmode: flex\ntitle: Doc\nnumbering:\n  enabled: true\n  style: 1.1.1\n---\n\nContenido.")
	for _, d := range diags {
		t.Logf("diagnostic: %v", d)
	}
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.Numbering == nil {
		t.Fatal("Numbering = nil, want a non-nil pointer to true")
	}
	if *node.Numbering != true {
		t.Errorf("Numbering = %v, want true", *node.Numbering)
	}
}

// TestFrontMatterParser_NumberingLegacyMapEnabledFalse is the symmetric
// legacy-map case with `enabled: false`.
func TestFrontMatterParser_NumberingLegacyMapEnabledFalse(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, _ := p.Parse("---\nmode: flex\ntitle: Doc\nnumbering:\n  enabled: false\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.Numbering == nil {
		t.Fatal("Numbering = nil, want a non-nil pointer to false")
	}
	if *node.Numbering != false {
		t.Errorf("Numbering = %v, want false", *node.Numbering)
	}
}

// TestFrontMatterParser_NumberingLegacyMapEnabledMissing covers the legacy
// map form with no `enabled` key at all (`numbering: {}` or a map that only
// sets `style`). That shape carries no information about intent, so it
// resolves to nil — the same as `numbering:` being absent entirely — rather
// than an implicit `false`, matching how a caller who wrote `numbering:` at
// all almost certainly meant to say *something*, but "the map exists" is not
// itself that something.
func TestFrontMatterParser_NumberingLegacyMapEnabledMissing(t *testing.T) {
	p := &FrontMatterParser{}

	node, _, _ := p.Parse("---\nmode: flex\ntitle: Doc\nnumbering:\n  style: 1.1.1\n---\n\nContenido.")
	if node == nil {
		t.Fatal("node should not be nil")
	}
	if node.Numbering != nil {
		t.Errorf("Numbering = %v, want nil when the map omits 'enabled'", *node.Numbering)
	}
}
