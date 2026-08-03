package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestE2E_WaiverSurvivesFmtRoundTrip is E2E scenario #1 from
// 2026-07-plan-toolchain-registry-waivers-reporte.md §8, marked
// "obligatorio, no opcional": a `lint_policy:` waiver embedded in
// frontmatter must survive build -> fmt --write -> build. This guards
// against the failure class of issue #238 (`@include` got canonicalized by
// `fmt` into a form the expander no longer recognized) — verified NOT to be
// broken today (core/formatter/frontmatter.go re-parses the raw frontmatter
// into a generic map rather than reconstructing from typed fields), but
// nothing locked that in before this test.
//
// "Survives" is checked SEMANTICALLY, never byte-for-byte: fmt re-orders
// frontmatter keys alphabetically and re-quotes values, so an exact-bytes
// comparison would fail for a reason unrelated to the guarantee this test
// protects.
func TestE2E_WaiverSurvivesFmtRoundTrip(t *testing.T) {
	dir := t.TempDir()
	rulepack := writeToyRulepack(t, dir)
	fixture := writeFixture(t, dir, true)
	outDir := filepath.Join(dir, "out")

	if err := runCLI(t, Options{ExternalRulepacks: []string{rulepack}},
		[]string{"build", fixture, "--format", "html", "--output", outDir}); err != nil {
		t.Fatalf("build with waiver present failed: %v", err)
	}

	beforePolicy := resolvePolicyFromFile(t, fixture)
	beforeBytes, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("failed to read fixture before fmt: %v", err)
	}

	if err := runCLI(t, Options{}, []string{"fmt", fixture, "--write"}); err != nil {
		t.Fatalf("fmt --write failed: %v", err)
	}

	// fmt --write is a no-op when its output is byte-identical to the input
	// (both CLIs' runFmt short-circuit on out == string(content)) — so this
	// proves fmt actually rewrote the file instead of silently no-op'ing,
	// which would make the "survives" claim below vacuous.
	afterBytes, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("failed to read fixture after fmt: %v", err)
	}
	if string(beforeBytes) == string(afterBytes) {
		t.Fatal("fmt --write did not change the fixture — this round-trip test would be vacuous")
	}

	afterPolicy := resolvePolicyFromFile(t, fixture)

	if !reflect.DeepEqual(beforePolicy.Rules["EXT001"], afterPolicy.Rules["EXT001"]) {
		t.Errorf("waiver for EXT001 changed across the fmt round-trip:\nbefore: %+v\nafter:  %+v",
			beforePolicy.Rules["EXT001"], afterPolicy.Rules["EXT001"])
	}

	if err := runCLI(t, Options{ExternalRulepacks: []string{rulepack}},
		[]string{"build", fixture, "--format", "html", "--output", outDir}); err != nil {
		t.Fatalf("build after fmt round-trip failed (waiver did not survive): %v", err)
	}
}

// TestE2E_WaiverSurvivesFmtRoundTrip_NegativeControl is the mandatory
// negative control for the test above: without any lint_policy waiver, the
// same toy rulepack finding must fail the build both before and after fmt.
// Without this control, a silently-not-running rulepack (e.g. a broken
// script path) would make the positive test pass for the wrong reason.
func TestE2E_WaiverSurvivesFmtRoundTrip_NegativeControl(t *testing.T) {
	dir := t.TempDir()
	rulepack := writeToyRulepack(t, dir)
	fixture := writeFixture(t, dir, false)
	outDir := filepath.Join(dir, "out")

	if err := runCLI(t, Options{ExternalRulepacks: []string{rulepack}},
		[]string{"build", fixture, "--format", "html", "--output", outDir}); err == nil {
		t.Fatal("expected build to fail without a waiver (unwaived EXT001 is an Error), got nil")
	}

	if err := runCLI(t, Options{}, []string{"fmt", fixture, "--write"}); err != nil {
		t.Fatalf("fmt --write failed: %v", err)
	}

	if err := runCLI(t, Options{ExternalRulepacks: []string{rulepack}},
		[]string{"build", fixture, "--format", "html", "--output", outDir}); err == nil {
		t.Fatal("expected build to still fail after fmt round-trip without a waiver, got nil")
	}
}
