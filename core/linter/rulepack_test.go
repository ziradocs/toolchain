package linter

import (
	"testing"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

type dummyRule struct{}

func (d *dummyRule) ID() string {
	return "DUMMY001"
}

func (d *dummyRule) Check(node ast.Node) []diagnostics.Diagnostic {
	return []diagnostics.Diagnostic{
		diagnostics.NewError("dummy", diagnostics.Position{}, "dummy").WithRuleID("DUMMY001"),
	}
}

type dummyRulepack struct{}

func (d *dummyRulepack) Name() string {
	return "dummy-pack"
}

func (d *dummyRulepack) Rules() []Rule {
	return []Rule{&dummyRule{}}
}

func TestLinter_WithRulePacks(t *testing.T) {
	l := NewWithRules()

	pack := &dummyRulepack{}
	for _, r := range pack.Rules() {
		l.AddRule(r)
	}

	doc := ast.NewAST(diagnostics.Position{})
	diags := l.LintUnfiltered(doc)

	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].RuleID != "DUMMY001" {
		t.Errorf("expected DUMMY001, got %s", diags[0].RuleID)
	}
}
