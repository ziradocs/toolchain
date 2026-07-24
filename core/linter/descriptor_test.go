// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"reflect"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

type describableRule struct {
	id string
}

func (r *describableRule) Check(node ast.Node) []diagnostics.Diagnostic {
	return nil
}

func (r *describableRule) Descriptors() []RuleDescriptor {
	return []RuleDescriptor{{ID: r.id, HelpURI: "https://example.com/" + r.id}}
}

type plainRule struct{}

func (r *plainRule) Check(node ast.Node) []diagnostics.Diagnostic {
	return nil
}

type describablePack struct {
	name  string
	rules []Rule
}

func (p *describablePack) Name() string {
	return p.name
}

func (p *describablePack) Rules() []Rule {
	return p.rules
}

func (p *describablePack) Descriptors() []RuleDescriptor {
	return []RuleDescriptor{{ID: "PACK001", Name: "Pack-level descriptor"}}
}

type plainPack struct {
	rules []Rule
}

func (p *plainPack) Name() string {
	return "plain-pack"
}

func (p *plainPack) Rules() []Rule {
	return p.rules
}

func TestCollectDescriptors_FromRules(t *testing.T) {
	rules := []Rule{
		&describableRule{id: "RULE001"},
		&plainRule{},
	}

	got := CollectDescriptors(rules, nil)
	if len(got) != 1 || got[0].ID != "RULE001" {
		t.Fatalf("expected 1 descriptor for RULE001, got %+v", got)
	}
}

func TestCollectDescriptors_PackLevelSurvivesFlattening(t *testing.T) {
	pack := &describablePack{
		name:  "wcag-pack",
		rules: []Rule{&describableRule{id: "WCAG001"}, &plainRule{}},
	}

	got := CollectDescriptors(nil, []RulePack{pack})

	ids := map[string]bool{}
	for _, d := range got {
		ids[d.ID] = true
	}

	if !ids["PACK001"] {
		t.Errorf("expected pack-level descriptor PACK001 to survive, got %+v", got)
	}
	if !ids["WCAG001"] {
		t.Errorf("expected rule-level descriptor WCAG001 from inside the pack, got %+v", got)
	}
}

func TestCollectDescriptors_PlainPackNoDescriptors(t *testing.T) {
	pack := &plainPack{rules: []Rule{&plainRule{}}}

	got := CollectDescriptors(nil, []RulePack{pack})
	if len(got) != 0 {
		t.Errorf("expected no descriptors from a pack/rules that don't implement Describable, got %+v", got)
	}
}

func TestNormalizeDescriptors_SortsDedupsAndDropsEmptyID(t *testing.T) {
	in := []RuleDescriptor{
		{ID: "ZZZ001", Name: "first"},
		{ID: "", Name: "no id, dropped"},
		{ID: "AAA001", Name: "second"},
		{ID: "ZZZ001", Name: "last wins"},
	}

	got := NormalizeDescriptors(in)

	want := []RuleDescriptor{
		{ID: "AAA001", Name: "second"},
		{ID: "ZZZ001", Name: "last wins"},
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("NormalizeDescriptors() = %+v, want %+v", got, want)
	}
}

func TestNormalizeDescriptors_OrderIndependent(t *testing.T) {
	a := []RuleDescriptor{{ID: "C001"}, {ID: "A001"}, {ID: "B001"}}
	b := []RuleDescriptor{{ID: "B001"}, {ID: "A001"}, {ID: "C001"}}

	if !reflect.DeepEqual(NormalizeDescriptors(a), NormalizeDescriptors(b)) {
		t.Errorf("expected NormalizeDescriptors to be independent of input order")
	}
}
