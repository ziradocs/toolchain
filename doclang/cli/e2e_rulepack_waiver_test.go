package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.ziradocs.com/core/v2/report"
)

// TestE2E_ExternalRulepackFindingIsWaivable is E2E scenario #3: an external
// rulepack's finding must be mergeable with in-process diagnostics BEFORE
// policy.Evaluate runs, so it can be waived like any other finding — the
// "punto crítico de integración" §7 of the plan calls out ("si se
// mezclaran después, un hallazgo de un pack no podría llevar waiver, y eso
// rompería el producto entero"). The build's exit status IS the assertion:
// both CLIs abort on any remaining Error-severity diagnostic AFTER waivers
// are applied.
func TestE2E_ExternalRulepackFindingIsWaivable(t *testing.T) {
	dir := t.TempDir()
	rulepack := writeToyRulepack(t, dir)

	unwaived := writeFixture(t, dir, false)
	if err := runCLI(t, Options{ExternalRulepacks: []string{rulepack}},
		[]string{"build", unwaived, "--format", "html", "--output", filepath.Join(dir, "out1")}); err == nil {
		t.Fatal("expected build to fail: external rulepack finding EXT001 (Error) has no waiver")
	}

	waived := writeFixture(t, dir, true)
	if err := runCLI(t, Options{ExternalRulepacks: []string{rulepack}},
		[]string{"build", waived, "--format", "html", "--output", filepath.Join(dir, "out2")}); err != nil {
		t.Fatalf("expected build to pass: EXT001 is waived, got error: %v", err)
	}
}

// TestE2E_RulepackFindingWaivedByMatchingScope isolates `scope` matching
// from the primary (unscoped) waiver fixture: matchScopeGlob does its own
// glob->regex conversion, and a scope surprise would otherwise be
// indistinguishable from a formatter regression in the round-trip test.
// Uses each fixture's own exact absolute path as the scope entry (no
// wildcards needed to prove the match/no-match branches).
func TestE2E_RulepackFindingWaivedByMatchingScope(t *testing.T) {
	dir := t.TempDir()
	rulepack := writeToyRulepack(t, dir)

	matching := writeFixtureScoped(t, dir, "matching.doclang", true)
	if err := runCLI(t, Options{ExternalRulepacks: []string{rulepack}},
		[]string{"build", matching, "--format", "html", "--output", filepath.Join(dir, "out-match")}); err != nil {
		t.Fatalf("expected build to pass: scope matches the fixture's own absolute path, got error: %v", err)
	}

	nonMatching := writeFixtureScoped(t, dir, "other.doclang", false)
	if err := runCLI(t, Options{ExternalRulepacks: []string{rulepack}},
		[]string{"build", nonMatching, "--format", "html", "--output", filepath.Join(dir, "out-nomatch")}); err == nil {
		t.Fatal("expected build to fail: waiver's scope does not match this fixture's path")
	}
}

// TestE2E_SARIFReportReflectsRulepackFindingAndWaiver is E2E scenario #2:
// unlike core/report/report_test.go's TestWriteReport_SARIFSchemaValid
// (which calls WriteReport directly with synthetic diagnostics and an empty
// waived slice), this exercises the real CLI wiring end-to-end — an
// external rulepack's finding and descriptor reaching the SARIF file the
// `build --report sarif` flag actually produces, unwaived and waived.
func TestE2E_SARIFReportReflectsRulepackFindingAndWaiver(t *testing.T) {
	dir := t.TempDir()
	rulepack := writeToyRulepack(t, dir)

	// Unwaived: the build fails, but the report must still have been
	// written — both CLIs call report.WriteReport before the
	// severity-abort check.
	unwaived := writeFixture(t, dir, false)
	unwaivedReportPath := filepath.Join(dir, "unwaived.sarif")
	_ = runCLI(t, Options{ExternalRulepacks: []string{rulepack}},
		[]string{"build", unwaived, "--format", "html", "--output", filepath.Join(dir, "out1"),
			"--report", "sarif", "--report-out", unwaivedReportPath})

	sarif := readSARIF(t, unwaivedReportPath)
	if !sarifHasRuleDescriptor(sarif, "EXT001") {
		t.Error("expected driver.rules[] to contain the EXT001 descriptor from the external rulepack")
	}
	result := findSARIFResult(sarif, "EXT001")
	if result == nil {
		t.Fatal("expected results[] to contain EXT001")
	}
	if len(result.Suppressions) != 0 {
		t.Errorf("expected no suppressions for the unwaived finding, got %+v", result.Suppressions)
	}

	// Waived: same finding, now suppressed with the waiver's justification.
	waived := writeFixture(t, dir, true)
	waivedReportPath := filepath.Join(dir, "waived.sarif")
	if err := runCLI(t, Options{ExternalRulepacks: []string{rulepack}},
		[]string{"build", waived, "--format", "html", "--output", filepath.Join(dir, "out2"),
			"--report", "sarif", "--report-out", waivedReportPath}); err != nil {
		t.Fatalf("build with waiver should pass: %v", err)
	}

	sarif = readSARIF(t, waivedReportPath)
	result = findSARIFResult(sarif, "EXT001")
	if result == nil {
		t.Fatal("expected results[] to contain EXT001")
	}
	if len(result.Suppressions) != 1 {
		t.Fatalf("expected exactly one suppression for the waived finding, got %d", len(result.Suppressions))
	}
	sup := result.Suppressions[0]
	if sup.Kind != "external" || sup.Status != "accepted" {
		t.Errorf("unexpected suppression kind/status: %+v", sup)
	}
	if sup.Justification != "known-good fixture for the E2E waiver guard" {
		t.Errorf("expected suppression justification to be the waiver's reason, got %q", sup.Justification)
	}
}

func readSARIF(t *testing.T, path string) report.SARIF {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read SARIF report: %v", err)
	}
	var s report.SARIF
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("SARIF report is not valid JSON matching report.SARIF: %v", err)
	}
	return s
}

func sarifHasRuleDescriptor(s report.SARIF, id string) bool {
	if len(s.Runs) == 0 {
		return false
	}
	for _, r := range s.Runs[0].Tool.Driver.Rules {
		if r.Id == id {
			return true
		}
	}
	return false
}

func findSARIFResult(s report.SARIF, ruleID string) *report.Result {
	if len(s.Runs) == 0 {
		return nil
	}
	for i := range s.Runs[0].Results {
		if s.Runs[0].Results[i].RuleId == ruleID {
			return &s.Runs[0].Results[i]
		}
	}
	return nil
}
