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
