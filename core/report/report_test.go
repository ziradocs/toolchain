package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/linter"
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

	descriptors := []linter.RuleDescriptor{
		{ID: "IMG001", HelpURI: "https://example.com/rules/IMG001", Properties: map[string]any{"category": "images"}},
	}

	err := WriteReport("json", outPath, active, waived, descriptors, nil, nil, nil)
	if err != nil {
		t.Fatalf("WriteReport failed: %v", err)
	}

	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var res struct {
		ReportVersion string `json:"reportVersion"`
		Rules         []struct {
			ID      string `json:"id"`
			HelpURI string `json:"helpUri"`
		} `json:"rules"`
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

	if res.ReportVersion != "1.1.0" {
		t.Errorf("Expected reportVersion 1.1.0, got %q", res.ReportVersion)
	}

	if len(res.Rules) != 1 || res.Rules[0].ID != "IMG001" || res.Rules[0].HelpURI != "https://example.com/rules/IMG001" {
		t.Errorf("Expected 1 rule descriptor for IMG001 with helpUri, got %+v", res.Rules)
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

func TestWriteReport_JSON_NoDescriptors_OmitsRulesKey(t *testing.T) {
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "report.json")

	active := []diagnostics.Diagnostic{
		diagnostics.NewError("error msg", diagnostics.Position{Line: 1}, "linter").WithRuleID("IMG001"),
	}

	err := WriteReport("json", outPath, active, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("WriteReport failed: %v", err)
	}

	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var res map[string]interface{}
	if err := json.Unmarshal(b, &res); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if _, ok := res["rules"]; ok {
		t.Errorf("Expected no 'rules' key when no descriptors are supplied, got %v", res["rules"])
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

	descriptors := []linter.RuleDescriptor{
		{ID: "IMG001", HelpURI: "https://example.com/rules/IMG001"},
		{ID: "IMG002", Name: "Image missing source"},
	}

	err := WriteReport("sarif", outPath, active, waived, descriptors, nil, nil, nil)
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

	tool := run["tool"].(map[string]interface{})
	driver := tool["driver"].(map[string]interface{})
	driverRules, ok := driver["rules"].([]interface{})
	if !ok || len(driverRules) != 2 {
		t.Fatalf("Expected driver.rules[] with 2 descriptors, got %v", driver["rules"])
	}

	seenIDs := map[string]bool{}
	for _, r := range driverRules {
		rm := r.(map[string]interface{})
		seenIDs[rm["id"].(string)] = true
	}
	if !seenIDs["IMG001"] || !seenIDs["IMG002"] {
		t.Errorf("Expected driver.rules[] to contain IMG001 and IMG002, got %+v", driverRules)
	}
}

func TestWriteReport_SARIF_NoDescriptors_OmitsRulesKey(t *testing.T) {
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "report.sarif")

	active := []diagnostics.Diagnostic{
		diagnostics.NewError("error msg", diagnostics.Position{Line: 1}, "linter").WithRuleID("IMG001"),
	}

	err := WriteReport("sarif", outPath, active, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("WriteReport failed: %v", err)
	}

	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	var sarif map[string]interface{}
	if err := json.Unmarshal(b, &sarif); err != nil {
		t.Fatalf("Unmarshal SARIF failed: %v", err)
	}

	run := sarif["runs"].([]interface{})[0].(map[string]interface{})
	driver := run["tool"].(map[string]interface{})["driver"].(map[string]interface{})

	if _, ok := driver["rules"]; ok {
		t.Errorf("Expected no driver.rules key when no descriptors are supplied, got %v", driver["rules"])
	}
}

func TestWriteReport_Descriptors_DeterministicOrder(t *testing.T) {
	active := []diagnostics.Diagnostic{
		diagnostics.NewError("error msg", diagnostics.Position{Line: 1}, "linter").WithRuleID("IMG001"),
	}

	forward := []linter.RuleDescriptor{
		{ID: "AAA001", HelpURI: "https://example.com/a"},
		{ID: "ZZZ001", HelpURI: "https://example.com/z"},
		{ID: "MMM001", HelpURI: "https://example.com/m"},
	}
	reversed := []linter.RuleDescriptor{forward[2], forward[0], forward[1]}

	tempDir := t.TempDir()
	outA := filepath.Join(tempDir, "a.sarif")
	outB := filepath.Join(tempDir, "b.sarif")

	if err := WriteReport("sarif", outA, active, nil, linter.NormalizeDescriptors(forward), nil, nil, nil); err != nil {
		t.Fatalf("WriteReport failed: %v", err)
	}
	if err := WriteReport("sarif", outB, active, nil, linter.NormalizeDescriptors(reversed), nil, nil, nil); err != nil {
		t.Fatalf("WriteReport failed: %v", err)
	}

	bA, _ := os.ReadFile(outA)
	bB, _ := os.ReadFile(outB)

	if string(bA) != string(bB) {
		t.Errorf("Expected byte-identical SARIF output regardless of descriptor input order")
	}
}

func TestWriteReport_UnknownFormat(t *testing.T) {
	err := WriteReport("xml", "out.xml", []diagnostics.Diagnostic{}, nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("Expected error for unknown format, got nil")
	}
}

func TestWriteReport_SARIFSchemaValid(t *testing.T) {
	active := []diagnostics.Diagnostic{
		{
			Code:     "TEST001",
			Severity: diagnostics.Error,
			Message:  "Test error",
			Source:   "test-rule",
			Position: diagnostics.Position{Line: 1, Column: 2},
		},
	}
	waived := []linter.WaivedDiagnostic{}
	descriptors := []linter.RuleDescriptor{
		{
			ID:         "TEST001",
			Name:       "Test rule",
			HelpURI:    "https://example.com/rules/TEST001",
			Properties: map[string]any{"category": "test"},
		},
	}

	outPath := filepath.Join(t.TempDir(), "report.sarif")
	err := WriteReport("sarif", outPath, active, waived, descriptors, nil, nil, nil)
	if err != nil {
		t.Fatalf("WriteReport failed: %v", err)
	}

	content, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("Failed to read generated report: %v", err)
	}

	compiler := jsonschema.NewCompiler()
	schemaPath := filepath.Join("testdata", "sarif-schema-2.1.0.json")
	sch, err := compiler.Compile(schemaPath)
	if err != nil {
		t.Fatalf("Failed to compile vendored SARIF 2.1.0 schema: %v", err)
	}

	var v interface{}
	if err := json.Unmarshal(content, &v); err != nil {
		t.Fatalf("Invalid JSON in SARIF report: %v", err)
	}

	if err := sch.Validate(v); err != nil {
		t.Errorf("SARIF output (with driver.rules[] populated) does not match schema: %v", err)
	}
}
