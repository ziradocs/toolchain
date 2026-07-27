package linter

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func TestExternalRulepack(t *testing.T) {
	// Create a temporary bash script that acts as an external rulepack
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "fake-rulepack.sh")

	// The script will just output a fixed JSON matching externalReport
	scriptContent := `#!/bin/bash
cat << 'EOF'
{
  "reportVersion": "1.0",
  "manifest": {
    "name": "fake-pack",
    "version": "1.2.3",
    "prefix": "EXT"
  },
  "findings": [
    {
      "code": "EXT001",
      "severity": "error",
      "message": "External finding",
      "position": {
        "line": 42
      }
    }
  ]
}
EOF
`
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		t.Fatalf("Failed to write fake rulepack: %v", err)
	}

	l := NewWithRules() // Empty rules
	l.WithRulepacks([]string{scriptPath}, 5*time.Second)

	doc := &ast.AST{
		FilePath: "test.slidelang",
	}

	diags := l.LintUnfiltered(doc)

	if len(diags) != 1 {
		t.Fatalf("Expected 1 diagnostic, got %d", len(diags))
	}

	if diags[0].Code != "EXT001" {
		t.Errorf("Expected code EXT001, got %s", diags[0].Code)
	}
	if diags[0].Severity != diagnostics.Error {
		t.Errorf("Expected ERROR severity, got %s", diags[0].Severity)
	}
	if diags[0].Source != "fake-pack@1.2.3" {
		t.Errorf("Expected fake-pack@1.2.3 source, got %s", diags[0].Source)
	}
}

// TestExternalRulepack_ReceivesThemeVariablesViaEnvVar cubre el canal de
// tema para rulepacks externos (issue #30, Part 3): el subproceso no puede
// ver el mapa de variables por stdin (ese envelope es intencionalmente el
// AST desnudo, sin cambios), así que WithThemeVariables debe llegarle vía
// la variable de entorno ZIRADOCS_THEME_VARIABLES como JSON. El script fake
// la lee y la devuelve dentro del mensaje del finding, para verificar que
// llegó intacta y es JSON parseable.
func TestExternalRulepack_ReceivesThemeVariablesViaEnvVar(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "fake-theme-rulepack.sh")

	scriptContent := `#!/bin/bash
# Escape double quotes so the theme JSON can be embedded as a JSON string
# VALUE below (it arrives as raw JSON, e.g. {"--text-color":"#111111"}).
escaped=$(printf '%s' "$ZIRADOCS_THEME_VARIABLES" | sed 's/"/\\"/g')
cat << EOF
{
  "reportVersion": "1.0",
  "manifest": {
    "name": "fake-theme-pack",
    "version": "1.0.0",
    "prefix": "EXT"
  },
  "findings": [
    {
      "code": "EXT002",
      "severity": "warning",
      "message": "theme=${escaped}",
      "position": {
        "line": 1
      }
    }
  ]
}
EOF
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write fake rulepack: %v", err)
	}

	l := NewWithRules()
	l.WithRulepacks([]string{scriptPath}, 5*time.Second)
	l.WithThemeVariables(map[string]string{"--text-color": "#111111"})

	doc := &ast.AST{FilePath: "test.slidelang"}
	diags := l.LintUnfiltered(doc)

	if len(diags) != 1 {
		t.Fatalf("Expected 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	want := `theme={"--text-color":"#111111"}`
	if diags[0].Message != want {
		t.Errorf("Message = %q, want %q (theme variables must arrive via %s)", diags[0].Message, want, themeVariablesEnvVar)
	}
}

// TestExternalRulepack_StaleInheritedEnvVarIsCleared es la regresión para
// el hallazgo del code review: si el proceso PADRE ya tenía
// ZIRADOCS_THEME_VARIABLES en su entorno (p. ej. un wrapper de CI, o un
// build anterior en el mismo proceso embebido) pero ESTA corrida nunca
// llamó WithThemeVariables, el rulepack no debe heredar ese valor viejo y
// atribuirle esos colores al documento actual — debe verla ausente, igual
// que en el caso histórico sin ningún valor puesto nunca.
func TestExternalRulepack_StaleInheritedEnvVarIsCleared(t *testing.T) {
	t.Setenv(themeVariablesEnvVar, `{"--stale-from-parent":"#dead00"}`)

	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "fake-stale-env-rulepack.sh")
	scriptContent := `#!/bin/bash
if [ -n "${ZIRADOCS_THEME_VARIABLES+x}" ]; then
  echo "unexpected: ZIRADOCS_THEME_VARIABLES is set to '${ZIRADOCS_THEME_VARIABLES}'" >&2
  exit 1
fi
cat << 'EOF'
{
  "reportVersion": "1.0",
  "manifest": {"name": "fake-pack", "version": "1.0.0", "prefix": "EXT"},
  "findings": []
}
EOF
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write fake rulepack: %v", err)
	}

	l := NewWithRules()
	l.WithRulepacks([]string{scriptPath}, 5*time.Second)
	// WithThemeVariables is deliberately never called on this Linter.

	doc := &ast.AST{FilePath: "test.slidelang"}
	diags := l.LintUnfiltered(doc)
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics (and no LINTER_SYS_ERR from the script's own exit 1), got %+v", diags)
	}
}

// TestExternalRulepack_NilThemeVariablesMarshalsToEmptyObject es la
// regresión para el segundo hallazgo del canal de subproceso: un tema
// legítimamente sin variables (WithThemeVariables(nil), un valor que la
// propia documentación de WithThemeVariables bendice explícitamente) debe
// serializarse como el objeto JSON vacío "{}", no como el literal "null" —
// un rulepack que hace json.Unmarshal a map[string]string o a un struct no
// debe encontrarse con una forma que no es un objeto.
func TestExternalRulepack_NilThemeVariablesMarshalsToEmptyObject(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "fake-nil-theme-rulepack.sh")
	scriptContent := `#!/bin/bash
escaped=$(printf '%s' "$ZIRADOCS_THEME_VARIABLES" | sed 's/"/\\"/g')
cat << EOF
{
  "reportVersion": "1.0",
  "manifest": {"name": "fake-pack", "version": "1.0.0", "prefix": "EXT"},
  "findings": [
    {"code": "EXT003", "severity": "warning", "message": "theme=${escaped}", "position": {"line": 1}}
  ]
}
EOF
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write fake rulepack: %v", err)
	}

	l := NewWithRules()
	l.WithRulepacks([]string{scriptPath}, 5*time.Second)
	l.WithThemeVariables(nil)

	doc := &ast.AST{FilePath: "test.slidelang"}
	diags := l.LintUnfiltered(doc)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d: %+v", len(diags), diags)
	}
	want := "theme={}"
	if diags[0].Message != want {
		t.Errorf("Message = %q, want %q (a nil theme variables map must marshal to an empty JSON object)", diags[0].Message, want)
	}
}

// TestExternalRulepack_NoThemeVariables_EnvVarAbsent confirma que un
// rulepack existente, que nunca lee ZIRADOCS_THEME_VARIABLES, sigue
// comportándose exactamente igual cuando WithThemeVariables nunca se
// llamó (el caso histórico) — no debería ni ver la variable definida.
func TestExternalRulepack_NoThemeVariables_EnvVarAbsent(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "fake-no-theme-rulepack.sh")

	scriptContent := `#!/bin/bash
if [ -n "${ZIRADOCS_THEME_VARIABLES+x}" ]; then
  echo "unexpected: ZIRADOCS_THEME_VARIABLES is set" >&2
  exit 1
fi
cat << 'EOF'
{
  "reportVersion": "1.0",
  "manifest": {"name": "fake-pack", "version": "1.0.0", "prefix": "EXT"},
  "findings": []
}
EOF
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write fake rulepack: %v", err)
	}

	l := NewWithRules()
	l.WithRulepacks([]string{scriptPath}, 5*time.Second)

	doc := &ast.AST{FilePath: "test.slidelang"}
	diags := l.LintUnfiltered(doc)
	if len(diags) != 0 {
		t.Fatalf("expected no diagnostics (and no LINTER_SYS_ERR from the script's own exit 1), got %+v", diags)
	}
}

func TestExternalRulepack_WithDescriptors(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "described-pack.sh")

	scriptContent := `#!/bin/bash
cat << 'EOF'
{
  "reportVersion": "1.0",
  "manifest": {
    "name": "described-pack",
    "version": "2.0.0",
    "prefix": "DESC"
  },
  "findings": [],
  "descriptors": [
    {
      "id": "DESC001",
      "helpUri": "https://example.com/rules/DESC001",
      "properties": {"category": "contrast"}
    }
  ]
}
EOF
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write fake rulepack: %v", err)
	}

	l := NewWithRules()
	l.WithRulepacks([]string{scriptPath}, 5*time.Second)

	doc := &ast.AST{FilePath: "test.slidelang"}
	_, got := l.LintUnfilteredWithDescriptors(doc)

	if len(got) != 1 || got[0].ID != "DESC001" || got[0].HelpURI != "https://example.com/rules/DESC001" {
		t.Fatalf("Expected 1 descriptor DESC001 with helpUri, got %+v", got)
	}
	if got[0].Properties["category"] != "contrast" {
		t.Errorf("Expected property category=contrast, got %+v", got[0].Properties)
	}
}

func TestExternalRulepack_MalformedDescriptorsKeepFindings(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "bad-desc-pack.sh")

	// descriptors is malformed (an object where an array is expected). Los
	// findings reales NO deben perderse por eso.
	scriptContent := `#!/bin/bash
cat << 'EOF'
{
  "reportVersion": "1.0",
  "manifest": {"name": "bad-desc-pack", "version": "1.0.0", "prefix": "BAD"},
  "findings": [
    {"code": "BAD001", "severity": "error", "message": "real finding", "position": {"line": 7}}
  ],
  "descriptors": {"not": "an array"}
}
EOF
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write fake rulepack: %v", err)
	}

	l := NewWithRules()
	l.WithRulepacks([]string{scriptPath}, 5*time.Second)

	doc := &ast.AST{FilePath: "test.slidelang"}
	diags, descs := l.LintUnfilteredWithDescriptors(doc)

	if len(descs) != 0 {
		t.Errorf("Expected malformed descriptors to be dropped, got %+v", descs)
	}
	if len(diags) != 1 || diags[0].Code != "BAD001" {
		t.Fatalf("Expected the real finding BAD001 to survive a malformed descriptors block, got %+v", diags)
	}
}

func TestExternalRulepack_NonStringPropertyValueDecodes(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "num-prop-pack.sh")

	// A number in the property bag (natural for SARIF) must decode, not fail.
	scriptContent := `#!/bin/bash
cat << 'EOF'
{
  "reportVersion": "1.0",
  "manifest": {"name": "num-prop-pack", "version": "1.0.0", "prefix": "NUM"},
  "findings": [],
  "descriptors": [
    {"id": "NUM001", "properties": {"minContrast": 4.5}}
  ]
}
EOF
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write fake rulepack: %v", err)
	}

	l := NewWithRules()
	l.WithRulepacks([]string{scriptPath}, 5*time.Second)

	doc := &ast.AST{FilePath: "test.slidelang"}
	_, descs := l.LintUnfilteredWithDescriptors(doc)

	if len(descs) != 1 || descs[0].ID != "NUM001" {
		t.Fatalf("Expected descriptor NUM001 with a numeric property to decode, got %+v", descs)
	}
	if descs[0].Properties["minContrast"] != 4.5 {
		t.Errorf("Expected minContrast=4.5 (float64), got %v", descs[0].Properties["minContrast"])
	}
}

func TestExternalRulepack_NoDescriptors(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "fake-rulepack.sh")

	scriptContent := `#!/bin/bash
cat << 'EOF'
{
  "reportVersion": "1.0",
  "manifest": {
    "name": "fake-pack",
    "version": "1.2.3",
    "prefix": "EXT"
  },
  "findings": []
}
EOF
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write fake rulepack: %v", err)
	}

	l := NewWithRules()
	l.WithRulepacks([]string{scriptPath}, 5*time.Second)

	doc := &ast.AST{FilePath: "test.slidelang"}
	_, got := l.LintUnfilteredWithDescriptors(doc)

	if len(got) != 0 {
		t.Errorf("Expected no descriptors when the rulepack emits none, got %+v", got)
	}
}

func TestLintUnfilteredWithDescriptors_ConcurrentReuse(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "concurrent-pack.sh")

	scriptContent := `#!/bin/bash
cat << 'EOF'
{
  "reportVersion": "1.0",
  "manifest": {"name": "concurrent-pack", "version": "1.0.0", "prefix": "CON"},
  "findings": [],
  "descriptors": [{"id": "CON001", "helpUri": "https://example.com/rules/CON001"}]
}
EOF
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("Failed to write fake rulepack: %v", err)
	}

	// Un solo *Linter compartido entre goroutines linteando documentos
	// distintos: LintUnfilteredWithDescriptors no debe mutar el receptor
	// (verificado además con -race).
	l := NewWithRules()
	l.WithRulepacks([]string{scriptPath}, 5*time.Second)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			doc := &ast.AST{FilePath: "concurrent.slidelang"}
			_, descs := l.LintUnfilteredWithDescriptors(doc)
			if len(descs) != 1 || descs[0].ID != "CON001" {
				t.Errorf("Expected exactly descriptor CON001 per call, got %+v", descs)
			}
		}()
	}
	wg.Wait()
}

func TestExternalRulepack_Timeout(t *testing.T) {
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "slow-pack.sh")

	scriptContent := `#!/bin/bash
sleep 2
`
	err := os.WriteFile(scriptPath, []byte(scriptContent), 0755)
	if err != nil {
		t.Fatalf("Failed to write fake rulepack: %v", err)
	}

	l := NewWithRules()
	l.WithRulepacks([]string{scriptPath}, 100*time.Millisecond) // Fast timeout

	doc := &ast.AST{}
	diags := l.LintUnfiltered(doc)

	if len(diags) != 1 {
		t.Fatalf("Expected exactly 1 diagnostic (the timeout error), got %d", len(diags))
	}

	if diags[0].Source != "LINTER_SYS_ERR" {
		t.Errorf("Expected LINTER_SYS_ERR source, got %s", diags[0].Source)
	}
}
