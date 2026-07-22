// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func intPtr(n int) *int { return &n }

// TestResolveLayoutSchema_NilPolicyReturnsBaseUnchanged y
// TestResolveLayoutSchema_UnknownLayoutTypeReturnsBaseUnchanged cubren los
// dos casos "no-op" de ResolveLayoutSchema antes de probar que realmente
// aplica overrides.
func TestResolveLayoutSchema_NilPolicyReturnsBaseUnchanged(t *testing.T) {
	base := SlideLayoutSchema{MinElements: 1, MaxElements: 8, ForbiddenElements: []string{"code"}}
	var policy *PolicyConfig

	got := policy.ResolveLayoutSchema("team", base)
	if got.MinElements != base.MinElements || got.MaxElements != base.MaxElements || len(got.ForbiddenElements) != 1 {
		t.Fatalf("nil policy should return base unchanged, got %+v", got)
	}
}

func TestResolveLayoutSchema_UnknownLayoutTypeReturnsBaseUnchanged(t *testing.T) {
	base := SlideLayoutSchema{MinElements: 1, MaxElements: 8}
	policy := &PolicyConfig{Layouts: map[string]LayoutOverride{
		"team": {MaxElements: intPtr(12)},
	}}

	got := policy.ResolveLayoutSchema("content", base)
	if got.MaxElements != base.MaxElements {
		t.Fatalf("override for a different layout type should not apply, got MaxElements=%d, want %d", got.MaxElements, base.MaxElements)
	}
}

func TestResolveLayoutSchema_OverridesMinMaxAndForbidden(t *testing.T) {
	base := SlideLayoutSchema{
		MinElements:       1,
		MaxElements:       8,
		ForbiddenElements: []string{"code", "chart", "table"},
	}
	policy := &PolicyConfig{Layouts: map[string]LayoutOverride{
		"team": {
			MinElements:       intPtr(2),
			MaxElements:       intPtr(12),
			ForbiddenElements: []string{"code"},
		},
	}}

	got := policy.ResolveLayoutSchema("team", base)
	if got.MinElements != 2 {
		t.Errorf("MinElements = %d, want 2", got.MinElements)
	}
	if got.MaxElements != 12 {
		t.Errorf("MaxElements = %d, want 12", got.MaxElements)
	}
	if len(got.ForbiddenElements) != 1 || got.ForbiddenElements[0] != "code" {
		t.Errorf("ForbiddenElements = %v, want [code] (replaced, not merged)", got.ForbiddenElements)
	}
}

// TestResolveLayoutSchema_PartialOverride_LeavesOtherFieldsAtBase confirma
// que un override que solo toca MaxElements no pisa MinElements/
// ForbiddenElements del schema base (semántica "solo lo que aparece en el
// override", no "reemplazo total del schema").
func TestResolveLayoutSchema_PartialOverride_LeavesOtherFieldsAtBase(t *testing.T) {
	base := SlideLayoutSchema{
		MinElements:       1,
		MaxElements:       8,
		ForbiddenElements: []string{"code"},
	}
	policy := &PolicyConfig{Layouts: map[string]LayoutOverride{
		"team": {MaxElements: intPtr(12)},
	}}

	got := policy.ResolveLayoutSchema("team", base)
	if got.MinElements != 1 {
		t.Errorf("MinElements = %d, want unchanged 1", got.MinElements)
	}
	if len(got.ForbiddenElements) != 1 || got.ForbiddenElements[0] != "code" {
		t.Errorf("ForbiddenElements = %v, want unchanged [code]", got.ForbiddenElements)
	}
	if got.MaxElements != 12 {
		t.Errorf("MaxElements = %d, want overridden to 12", got.MaxElements)
	}
}

// TestLoadPolicyConfig_ValidatesLayoutOverrides cubre los 3 casos que
// LoadPolicyConfig rechaza para overrides de layout, mismo patrón que
// TestLoadPolicyConfig_ValidatesSeverity.
func TestLoadPolicyConfig_ValidatesLayoutOverrides(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{"negative min_elements", "layouts:\n  team:\n    min_elements: -1\n"},
		{"negative max_elements", "layouts:\n  team:\n    max_elements: -5\n"},
		{"min greater than max", "layouts:\n  team:\n    min_elements: 10\n    max_elements: 5\n"},
		// Hallazgo de code-review (PR #220): un override que solo toca UN
		// lado puede contradecir el schema base sin que el chequeo
		// original (que solo comparaba ambos campos DENTRO del mismo
		// override) lo detectara. El schema base "team"
		// (layout_validation.go) es MinElements:1, MaxElements:8.
		{"partial override (min only) exceeds base MaxElements", "layouts:\n  team:\n    min_elements: 10\n"},
		// El schema base "comparison" es MinElements:2, MaxElements:4.
		{"partial override (max only) is below base MinElements", "layouts:\n  comparison:\n    max_elements: 1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "policy.yml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadPolicyConfig(path); err == nil {
				t.Fatalf("expected an error for invalid layout override %q", tc.yaml)
			}
		})
	}
}

// TestLoadPolicyConfig_MinEqualsMax_IsValid confirma que min_elements ==
// max_elements no dispara la validación "min mayor que max" (caso límite,
// no error).
func TestLoadPolicyConfig_MinEqualsMax_IsValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yml")
	if err := os.WriteFile(path, []byte("layouts:\n  team:\n    min_elements: 5\n    max_elements: 5\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPolicyConfig(path); err != nil {
		t.Fatalf("min_elements == max_elements should be valid, got error: %v", err)
	}
}

// TestLoadPolicyConfig_PartialOverride_ConsistentWithBase_IsValid confirma
// el complemento positivo de los 2 casos negativos agregados a
// TestLoadPolicyConfig_ValidatesLayoutOverrides: un override parcial que
// NO contradice el schema base (team: MinElements:1, MaxElements:8) pasa
// validación limpio.
func TestLoadPolicyConfig_PartialOverride_ConsistentWithBase_IsValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yml")
	if err := os.WriteFile(path, []byte("layouts:\n  team:\n    max_elements: 12\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPolicyConfig(path); err != nil {
		t.Fatalf("max_elements-only override consistent with base MinElements should be valid, got error: %v", err)
	}
}

// TestLoadPolicyConfig_MaxElementsZero_MeansUnlimited_NotAnError confirma
// que max_elements: 0 (explícito) no dispara la validación min>max incluso
// con un min_elements alto — 0 significa "ilimitado" en SlideLayoutSchema,
// no "cero", así que *override.MaxElements > 0 en la validación debe
// excluir este caso.
func TestLoadPolicyConfig_MaxElementsZero_MeansUnlimited_NotAnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.yml")
	if err := os.WriteFile(path, []byte("layouts:\n  team:\n    min_elements: 20\n    max_elements: 0\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadPolicyConfig(path)
	if err != nil {
		t.Fatalf("max_elements: 0 (unlimited) with any min_elements should be valid, got error: %v", err)
	}
	if cfg.Layouts["team"].MaxElements == nil || *cfg.Layouts["team"].MaxElements != 0 {
		t.Fatalf("expected MaxElements explicitly set to 0 (not nil), got %+v", cfg.Layouts["team"])
	}
}

// TestLinter_WithPolicy_LayoutMaxElementsOverride_EndToEnd es el test que
// cierra el loop completo de issue #207: un policy YAML real, cargado vía
// LoadPolicyConfig, atado a un Linter vía WithPolicy, cambiando qué
// diagnóstico produce SlideLayoutValidationRule.Check() -- no solo la
// función standalone ResolveLayoutSchema aislada. El schema "team" tiene
// MaxElements: 8 por defecto (ver layout_validation.go); este test
// construye un slide "team" con 10 elementos (excede el default, dentro de
// un override a 12) y confirma que el mensaje LAYOUT_MAX_ELEMENTS -- si
// llegara a dispararse por error -- también ejercitaría el fix de #200
// (strconv.Itoa) para un límite de dos dígitos.
func TestLinter_WithPolicy_LayoutMaxElementsOverride_EndToEnd(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	newTeamSlideWithNElements := func(n int) *ast.AST {
		astNode := ast.NewAST(pos)
		block := ast.NewContentBlock(pos, "team")
		block.Title = "Our Team"
		for i := 0; i < n; i++ {
			block.Elements = append(block.Elements, ast.NewImageElement(pos, "member.png", "Team member"))
		}
		astNode.ContentBlocks = append(astNode.ContentBlocks, *block)
		return astNode
	}

	// Baseline (sin policy): el schema "team" tiene MaxElements: 8, así que
	// 10 elementos deben producir LAYOUT_MAX_ELEMENTS.
	baseline := New().Lint(newTeamSlideWithNElements(10))
	if findDiagnostic(baseline, "LAYOUT_MAX_ELEMENTS") == nil {
		t.Fatalf("expected baseline lint (no policy) to report LAYOUT_MAX_ELEMENTS for 10 elements against the default MaxElements=8, got %+v", baseline)
	}

	// Con policy override a MaxElements: 12, los mismos 10 elementos ya no
	// deberían disparar el warning.
	policy := &PolicyConfig{Layouts: map[string]LayoutOverride{
		"team": {MaxElements: intPtr(12)},
	}}
	overridden := New().WithPolicy(policy).Lint(newTeamSlideWithNElements(10))
	if diag := findDiagnostic(overridden, "LAYOUT_MAX_ELEMENTS"); diag != nil {
		t.Fatalf("expected no LAYOUT_MAX_ELEMENTS with MaxElements overridden to 12 for 10 elements, got %+v", diag)
	}

	// 13 elementos SÍ deberían exceder el override de 12, y el mensaje debe
	// mostrar el texto decimal correcto (issue #200 vía el path real, no
	// solo el helper standalone).
	exceeded := New().WithPolicy(policy).Lint(newTeamSlideWithNElements(13))
	diag := findDiagnostic(exceeded, "LAYOUT_MAX_ELEMENTS")
	if diag == nil {
		t.Fatalf("expected LAYOUT_MAX_ELEMENTS for 13 elements against overridden MaxElements=12, got %+v", exceeded)
	}
	if !strings.Contains(diag.Message, "12") {
		t.Errorf("expected message to contain the decimal string \"12\", got %q", diag.Message)
	}
}

// TestLinter_WithPolicy_LayoutForbiddenElementsOverride_EndToEnd confirma
// que un override de ForbiddenElements cambia qué LAYOUT_FORBIDDEN_ELEMENT
// se produce -- el schema "content" no prohíbe "code" por defecto.
func TestLinter_WithPolicy_LayoutForbiddenElementsOverride_EndToEnd(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	astNode := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	block.Title = "Some Content"
	block.Elements = append(block.Elements, ast.NewCodeElement(pos, "go", "fmt.Println(\"hi\")"))
	astNode.ContentBlocks = append(astNode.ContentBlocks, *block)

	baseline := New().Lint(astNode)
	if findDiagnostic(baseline, "LAYOUT_FORBIDDEN_ELEMENT") != nil {
		t.Fatalf("expected no LAYOUT_FORBIDDEN_ELEMENT for 'code' in a 'content' slide by default, got %+v", baseline)
	}

	policy := &PolicyConfig{Layouts: map[string]LayoutOverride{
		"content": {ForbiddenElements: []string{"code"}},
	}}
	overridden := New().WithPolicy(policy).Lint(astNode)
	if findDiagnostic(overridden, "LAYOUT_FORBIDDEN_ELEMENT") == nil {
		t.Fatalf("expected LAYOUT_FORBIDDEN_ELEMENT after overriding 'content' to forbid 'code', got %+v", overridden)
	}
}
