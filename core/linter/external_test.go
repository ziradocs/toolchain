package linter

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
)

func TestExternalRulepack(t *testing.T) {
	// Create a temporary bash script that acts as an external rulepack
	tempDir := t.TempDir()
	scriptPath := filepath.Join(tempDir, "fake-rulepack.sh")

	// The script will just output a fixed JSON matching externalReport
	scriptContent := `#!/bin/bash
cat << 'EOF'
{
  "findings": [
    {
      "code": "EXT001",
      "severity": "error",
      "message": "External finding",
      "source": "fake-pack",
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
