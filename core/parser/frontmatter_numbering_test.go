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
