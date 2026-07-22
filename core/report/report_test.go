package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.ziradocs.com/core/diagnostics"
	"go.ziradocs.com/core/linter"
)

func TestWriteReport_JSON(t *testing.T) {
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "report.json")

	active := []diagnostics.Diagnostic{
		diagnostics.NewError("error msg", diagnostics.Position{Line: 1}, "linter").WithRuleID("IMG001"),
	}

	boolTrue := true
	waived := []linter.WaivedDiagnostic{
		{
			Diagnostic: diagnostics.NewWarning("warn msg", diagnostics.Position{Line: 2}, "linter").WithRuleID("IMG002"),
			Policy: &linter.RulePolicy{
				Enabled:   &boolTrue,
				ExpiresAt: "2026-01-01T00:00:00Z",
				Reason:    "legacy",
			},
		},
	}

	err := WriteReport("json", outPath, active, waived, "test.md")
	if err != nil {
		t.Fatalf("WriteReport failed: %v", err)
	}

	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var res struct {
		Findings []struct {
			RuleID   string `json:"ruleId"`
			Message  string `json:"message"`
			Severity string `json:"severity"`
			Waived   bool   `json:"waived"`
			Waiver   *struct {
				Reason string `json:"reason"`
			} `json:"waiver,omitempty"`
		} `json:"findings"`
	}

	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(res.Findings) != 2 {
		t.Fatalf("Expected 2 findings, got %d", len(res.Findings))
	}

	var activeCount, waivedCount int
	for _, f := range res.Findings {
		if !f.Waived && f.RuleID == "IMG001" {
			activeCount++
		} else if f.Waived && f.RuleID == "IMG002" && f.Waiver != nil && f.Waiver.Reason == "legacy" {
			waivedCount++
		}
	}

	if activeCount != 1 || waivedCount != 1 {
		t.Errorf("Expected 1 active IMG001 and 1 waived IMG002 legacy, got active=%d waived=%d", activeCount, waivedCount)
	}
}

func TestWriteReport_SARIF(t *testing.T) {
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "report.sarif")

	active := []diagnostics.Diagnostic{
		diagnostics.NewError("error msg", diagnostics.Position{Line: 1}, "linter").WithRuleID("IMG001"),
	}

	boolTrue := true
	waived := []linter.WaivedDiagnostic{
		{
			Diagnostic: diagnostics.NewWarning("warn msg", diagnostics.Position{Line: 2}, "linter").WithRuleID("IMG002"),
			Policy: &linter.RulePolicy{
				Enabled:   &boolTrue,
				ExpiresAt: "2026-01-01T00:00:00Z",
				Reason:    "legacy",
			},
		},
	}

	err := WriteReport("sarif", outPath, active, waived, "test.md")
	if err != nil {
		t.Fatalf("WriteReport failed: %v", err)
	}

	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	// Just a simple JSON unmarshal to check it's valid SARIF
	var sarif map[string]interface{}
	if err := json.Unmarshal(b, &sarif); err != nil {
		t.Fatalf("Unmarshal SARIF failed: %v", err)
	}

	if sarif["version"] != "2.1.0" {
		t.Errorf("Expected SARIF version 2.1.0, got %v", sarif["version"])
	}

	runs, ok := sarif["runs"].([]interface{})
	if !ok || len(runs) != 1 {
		t.Fatalf("Expected 1 run, got %v", runs)
	}

	run := runs[0].(map[string]interface{})
	results, ok := run["results"].([]interface{})
	if !ok || len(results) != 2 {
		t.Fatalf("Expected 2 results (1 active + 1 waived), got %d", len(results))
	}

	// Check suppressions
	hasSuppression := false
	for _, res := range results {
		resMap := res.(map[string]interface{})
		if supps, ok := resMap["suppressions"].([]interface{}); ok && len(supps) > 0 {
			hasSuppression = true
			supp := supps[0].(map[string]interface{})
			if supp["kind"] != "external" || supp["justification"] != "legacy" {
				t.Errorf("Expected suppression kind=external, justification=legacy, got %+v", supp)
			}
		}
	}

	if !hasSuppression {
		t.Errorf("Expected at least one suppression in results")
	}
}

func TestWriteReport_UnknownFormat(t *testing.T) {
	err := WriteReport("xml", "out.xml", []diagnostics.Diagnostic{}, nil, "test.md")
	if err == nil {
		t.Fatal("Expected error for unknown format, got nil")
	}
}
