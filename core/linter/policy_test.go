// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func boolPtr(b bool) *bool { return &b }

func TestPolicyConfig_Apply_DisablesRule(t *testing.T) {
	diags := []diagnostics.Diagnostic{
		diagnostics.NewError("img error", diagnostics.Position{}, "linter").WithRuleID("IMG001"),
		diagnostics.NewWarning("code warning", diagnostics.Position{}, "linter").WithRuleID("CODE001"),
	}
	cfg := &PolicyConfig{Rules: map[string]RulePolicy{
		"IMG001": {Enabled: boolPtr(false)},
	}}

	got := cfg.Apply(diags)
	if len(got) != 1 {
		t.Fatalf("expected 1 diagnostic after disabling IMG001, got %d: %+v", len(got), got)
	}
	if got[0].RuleID != "CODE001" {
		t.Errorf("expected surviving diagnostic to be CODE001, got %q", got[0].RuleID)
	}
}

func TestPolicyConfig_Apply_OverridesSeverity(t *testing.T) {
	diags := []diagnostics.Diagnostic{
		diagnostics.NewError("img error", diagnostics.Position{}, "linter").WithRuleID("IMG001"),
	}
	cfg := &PolicyConfig{Rules: map[string]RulePolicy{
		"IMG001": {Severity: "warning"},
	}}

	got := cfg.Apply(diags)
	if len(got) != 1 || got[0].Severity != diagnostics.Warning {
		t.Fatalf("expected IMG001 downgraded to warning, got %+v", got)
	}
}

func TestPolicyConfig_Apply_MatchesCodeFieldWhenRuleIDEmpty(t *testing.T) {
	// layout_validation.go asigna Code directamente en vez de usar WithRuleID
	// — Apply debe togglear por esa vía también (ver diagnosticRuleID).
	diags := []diagnostics.Diagnostic{
		{Severity: diagnostics.Warning, Message: "layout", Code: "LAYOUT001"},
	}
	cfg := &PolicyConfig{Rules: map[string]RulePolicy{
		"LAYOUT001": {Enabled: boolPtr(false)},
	}}

	got := cfg.Apply(diags)
	if len(got) != 0 {
		t.Fatalf("expected LAYOUT001 (matched via Code, not RuleID) to be filtered, got %+v", got)
	}
}

func TestPolicyConfig_Apply_NilPolicyIsNoOp(t *testing.T) {
	diags := []diagnostics.Diagnostic{
		diagnostics.NewError("x", diagnostics.Position{}, "linter").WithRuleID("IMG001"),
	}
	var cfg *PolicyConfig
	got := cfg.Apply(diags)
	if len(got) != 1 {
		t.Fatalf("nil policy should be a no-op, got %+v", got)
	}
}

func TestLoadPolicyConfig_EmptyPathReturnsNil(t *testing.T) {
	cfg, err := LoadPolicyConfig("")
	if err != nil || cfg != nil {
		t.Fatalf("expected nil, nil for empty path, got %+v, %v", cfg, err)
	}
}

func TestLoadPolicyConfig_ValidatesSeverity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yml")
	if err := os.WriteFile(path, []byte("rules:\n  IMG001:\n    severity: bogus\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPolicyConfig(path); err == nil {
		t.Fatal("expected an error for invalid severity value")
	}
}

func TestLoadPolicyConfig_LoadsAndApplies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yml")
	yml := "rules:\n  IMG001:\n    enabled: false\n  CODE001:\n    severity: error\n"
	if err := os.WriteFile(path, []byte(yml), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadPolicyConfig(path)
	if err != nil {
		t.Fatalf("LoadPolicyConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}

	diags := []diagnostics.Diagnostic{
		diagnostics.NewError("img", diagnostics.Position{}, "linter").WithRuleID("IMG001"),
		diagnostics.NewWarning("code", diagnostics.Position{}, "linter").WithRuleID("CODE001"),
	}
	got := cfg.Apply(diags)
	if len(got) != 1 {
		t.Fatalf("expected 1 diagnostic (IMG001 disabled), got %+v", got)
	}
	if got[0].RuleID != "CODE001" || got[0].Severity != diagnostics.Error {
		t.Fatalf("expected CODE001 upgraded to error, got %+v", got[0])
	}
}

// TestResolvePolicyConfig_FlagPathWins confirma la precedencia: si flagPath
// no está vacío, se usa (comportamiento idéntico a LoadPolicyConfig) y el
// frontmatter se ignora por completo, aunque tenga su propio lint_policy:.
func TestResolvePolicyConfig_FlagPathWins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yml")
	if err := os.WriteFile(path, []byte("rules:\n  IMG001:\n    enabled: false\n"), 0644); err != nil {
		t.Fatal(err)
	}

	fm := ast.NewFrontMatterNode(diagnostics.NewPosition(1, 1))
	fm.Raw = "title: Doc\nlint_policy:\n  rules:\n    CODE001:\n      enabled: false\n"

	cfg, err := ResolvePolicyConfig(path, fm)
	if err != nil {
		t.Fatalf("ResolvePolicyConfig: %v", err)
	}
	if _, ok := cfg.Rules["IMG001"]; !ok {
		t.Fatalf("expected the flag-path policy (IMG001) to win, got %+v", cfg.Rules)
	}
	if _, ok := cfg.Rules["CODE001"]; ok {
		t.Fatalf("expected the frontmatter policy (CODE001) to be ignored when flagPath is set, got %+v", cfg.Rules)
	}
}

// TestResolvePolicyConfig_NilFrontMatter_ReturnsNil y
// TestResolvePolicyConfig_NoLintPolicyKey_ReturnsNil cubren los 2 casos de
// "sin política" cuando no hay flag: sin frontmatter en absoluto, y
// frontmatter presente pero sin la clave lint_policy:.
func TestResolvePolicyConfig_NilFrontMatter_ReturnsNil(t *testing.T) {
	cfg, err := ResolvePolicyConfig("", nil)
	if err != nil || cfg != nil {
		t.Fatalf("expected nil, nil for nil frontmatter and no flag, got %+v, %v", cfg, err)
	}
}

func TestResolvePolicyConfig_NoLintPolicyKey_ReturnsNil(t *testing.T) {
	fm := ast.NewFrontMatterNode(diagnostics.NewPosition(1, 1))
	fm.Raw = "title: Doc\nmode: strict\ntheme: modern-blue\n"

	cfg, err := ResolvePolicyConfig("", fm)
	if err != nil || cfg != nil {
		t.Fatalf("expected nil, nil when frontmatter has no lint_policy key, got %+v, %v", cfg, err)
	}
}

// TestResolvePolicyConfig_InlineLintPolicy_ParsesAndApplies es el caso
// central de issue #208: un documento sin --lint-config pero con
// lint_policy: embebido en su propio frontmatter debe resolverse a un
// PolicyConfig funcional, parseado enteramente desde fm.Raw (sin tocar el
// filesystem).
func TestResolvePolicyConfig_InlineLintPolicy_ParsesAndApplies(t *testing.T) {
	fm := ast.NewFrontMatterNode(diagnostics.NewPosition(1, 1))
	fm.Raw = "title: Doc\nlint_policy:\n  rules:\n    IMG001:\n      enabled: false\n    CODE001:\n      severity: error\n"

	cfg, err := ResolvePolicyConfig("", fm)
	if err != nil {
		t.Fatalf("ResolvePolicyConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected a non-nil PolicyConfig")
	}

	diags := []diagnostics.Diagnostic{
		diagnostics.NewError("img", diagnostics.Position{}, "linter").WithRuleID("IMG001"),
		diagnostics.NewWarning("code", diagnostics.Position{}, "linter").WithRuleID("CODE001"),
	}
	got := cfg.Apply(diags)
	if len(got) != 1 || got[0].RuleID != "CODE001" || got[0].Severity != diagnostics.Error {
		t.Fatalf("expected IMG001 filtered and CODE001 upgraded to error, got %+v", got)
	}
}

// TestResolvePolicyConfig_InlineLintPolicy_InvalidSeverity_ReturnsError
// confirma que la política embebida pasa por la MISMA validación que
// LoadPolicyConfig (vía el parsePolicyConfig compartido) — un severity
// inválido en frontmatter falla igual que en un archivo.
func TestResolvePolicyConfig_InlineLintPolicy_InvalidSeverity_ReturnsError(t *testing.T) {
	fm := ast.NewFrontMatterNode(diagnostics.NewPosition(1, 1))
	fm.Raw = "title: Doc\nlint_policy:\n  rules:\n    IMG001:\n      severity: bogus\n"

	if _, err := ResolvePolicyConfig("", fm); err == nil {
		t.Fatal("expected an error for invalid severity in inline lint_policy")
	}
}

// TestResolvePolicyConfig_EndToEnd_ViaLinter cierra el loop completo: un
// AST real con frontmatter conteniendo lint_policy:, resuelto y atado a un
// Linter vía WithPolicy — no solo ResolvePolicyConfig aislado. Mismo
// espíritu que TestLinter_WithPolicy_IntegratesEndToEnd.
func TestResolvePolicyConfig_EndToEnd_ViaLinter(t *testing.T) {
	astNode := ast.NewAST(diagnostics.NewPosition(1, 1))
	astNode.FrontMatter = ast.NewFrontMatterNode(diagnostics.NewPosition(1, 1))
	astNode.FrontMatter.Raw = "title: Doc\nlint_policy:\n  rules:\n    IMG001:\n      enabled: false\n"
	astNode.ContentBlocks = append(astNode.ContentBlocks, *ast.NewContentBlock(diagnostics.NewPosition(1, 1), "content"))
	img := ast.NewImageElement(diagnostics.NewPosition(1, 1), "", "")
	astNode.ContentBlocks[0].Elements = append(astNode.ContentBlocks[0].Elements, img)

	hasIMG001 := func(diags []diagnostics.Diagnostic) bool {
		for _, d := range diags {
			if d.RuleID == "IMG001" {
				return true
			}
		}
		return false
	}

	baseline := New().Lint(astNode)
	if !hasIMG001(baseline) {
		t.Fatalf("expected baseline (no policy resolved yet) to report IMG001, got %+v", baseline)
	}

	resolved, err := ResolvePolicyConfig("", astNode.FrontMatter)
	if err != nil {
		t.Fatalf("ResolvePolicyConfig: %v", err)
	}
	filtered := New().WithPolicy(resolved).Lint(astNode)
	if hasIMG001(filtered) {
		t.Fatalf("expected IMG001 filtered by the frontmatter-embedded lint_policy, got %+v", filtered)
	}
}

// TestLinter_WithPolicy_IntegratesEndToEnd confirma que Linter.Lint()
// realmente aplica la política — no solo que PolicyConfig.Apply funciona
// aislado.
func TestLinter_WithPolicy_IntegratesEndToEnd(t *testing.T) {
	astNode := ast.NewAST(diagnostics.NewPosition(1, 1))
	astNode.ContentBlocks = append(astNode.ContentBlocks, *ast.NewContentBlock(diagnostics.NewPosition(1, 1), "content"))
	img := ast.NewImageElement(diagnostics.NewPosition(1, 1), "", "")
	astNode.ContentBlocks[0].Elements = append(astNode.ContentBlocks[0].Elements, img)

	baseline := New().Lint(astNode)
	foundIMG001 := false
	for _, d := range baseline {
		if d.RuleID == "IMG001" {
			foundIMG001 = true
		}
	}
	if !foundIMG001 {
		t.Fatalf("expected baseline lint to report IMG001 for an image with no source, got %+v", baseline)
	}

	policy := &PolicyConfig{Rules: map[string]RulePolicy{"IMG001": {Enabled: boolPtr(false)}}}
	filtered := New().WithPolicy(policy).Lint(astNode)
	for _, d := range filtered {
		if d.RuleID == "IMG001" {
			t.Fatalf("expected IMG001 to be filtered out by policy, got %+v", filtered)
		}
	}
}

func TestPolicyConfig_Evaluate_WaiverExpiration(t *testing.T) {
	diags := []diagnostics.Diagnostic{
		diagnostics.NewError("error", diagnostics.Position{}, "linter").WithRuleID("IMG001"),
	}
	cfg := &PolicyConfig{Rules: map[string]RulePolicy{
		"IMG001": {
			ExpiresAt: "2026-01-01T00:00:00Z",
			Reason:    "waiting for fix",
		},
	}}

	now := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	active, waived := cfg.Evaluate(diags, "test.md", now)

	if len(waived) != 0 {
		t.Fatalf("expected 0 waived diagnostics since waiver is expired, got %d", len(waived))
	}
	if len(active) != 2 {
		t.Fatalf("expected 2 active diagnostics (the original error + POLICY001), got %d", len(active))
	}

	hasPolicy001 := false
	hasIMG001 := false
	for _, d := range active {
		if d.RuleID == "IMG001" {
			hasIMG001 = true
		}
		if d.Code == "POLICY001" {
			hasPolicy001 = true
		}
	}
	if !hasPolicy001 || !hasIMG001 {
		t.Errorf("expected IMG001 and POLICY001 to be present, active=%+v", active)
	}
}

func TestPolicyConfig_Evaluate_WaiverValid(t *testing.T) {
	diags := []diagnostics.Diagnostic{
		diagnostics.NewError("error", diagnostics.Position{}, "linter").WithRuleID("IMG001"),
	}
	cfg := &PolicyConfig{Rules: map[string]RulePolicy{
		"IMG001": {
			ExpiresAt: "2026-01-01T00:00:00Z",
			Reason:    "waiting for fix",
		},
	}}

	now := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	active, waived := cfg.Evaluate(diags, "test.md", now)

	if len(active) != 0 {
		t.Fatalf("expected 0 active diagnostics since waiver is valid, got %d", len(active))
	}
	if len(waived) != 1 {
		t.Fatalf("expected 1 waived diagnostic, got %d", len(waived))
	}
}

func TestPolicyConfig_Evaluate_Scope(t *testing.T) {
	diags := []diagnostics.Diagnostic{
		diagnostics.NewError("error", diagnostics.Position{}, "linter").WithRuleID("IMG001"),
	}
	cfg := &PolicyConfig{Rules: map[string]RulePolicy{
		"IMG001": {
			ExpiresAt: "2026-01-01T00:00:00Z",
			Reason:    "waiting for fix",
			Scope:     []string{"foo/**/*.md"},
		},
	}}

	now := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)

	// out of scope
	active, waived := cfg.Evaluate(diags, "bar/test.md", now)
	if len(waived) != 0 || len(active) != 1 {
		t.Errorf("expected out of scope to not waive, active=%d waived=%d", len(active), len(waived))
	}

	// in scope
	active, waived = cfg.Evaluate(diags, "foo/bar/test.md", now)
	if len(waived) != 1 || len(active) != 0 {
		t.Errorf("expected in scope to waive, active=%d waived=%d", len(active), len(waived))
	}
}
