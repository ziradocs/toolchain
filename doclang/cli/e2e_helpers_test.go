package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/linter"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// runCLI builds a root command with opts and runs args against it in-process
// (no subprocess/binary), mirroring the existing e2e_test.go's pattern but
// using cmd.SetArgs instead of mutating the global os.Args, which would leak
// between tests running in the same package.
func runCLI(t *testing.T, opts Options, args []string) error {
	t.Helper()
	cmd := NewRootCommand(opts)
	cmd.SetArgs(args)
	// These E2E tests deliberately exercise failing builds (unwaived
	// findings); cobra's default behavior prints the full flag usage to
	// stderr on any RunE error, which is only noise here.
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	return cmd.Execute()
}

// toyRulepackScript is the fixed external-rulepack subprocess used by every
// E2E scenario in this file group: it always emits one Error-severity
// finding with code EXT001 (plus a matching descriptor), so every test
// shares a single deterministic source of a lintable violation instead of
// depending on any specific built-in rule's behavior. Bash, following
// core/linter/external_test.go's own fixture pattern.
const toyRulepackScript = `#!/bin/bash
cat << 'EOF'
{
  "reportVersion": "1.1.0",
  "manifest": {"name": "toy-rulepack", "version": "0.0.1", "prefix": "EXT"},
  "findings": [
    {"severity": "error", "message": "toy finding", "position": {"line": 1, "column": 1}, "code": "EXT001"}
  ],
  "descriptors": [{"id": "EXT001", "name": "Toy rule", "helpUri": "https://example.invalid/EXT001"}]
}
EOF
`

// writeToyRulepack writes toyRulepackScript to dir and returns its path.
// Skips the test on Windows (a platform this toolchain publishes per
// .goreleaser.yaml, but /bin/bash isn't available there) — same limitation
// core/linter/external_test.go's own bash fixtures already accept.
func writeToyRulepack(t *testing.T, dir string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("toy rulepack fixture is a bash script; not runnable on windows")
	}
	path := filepath.Join(dir, "toy-rulepack.sh")
	if err := os.WriteFile(path, []byte(toyRulepackScript), 0755); err != nil {
		t.Fatalf("failed to write toy rulepack: %v", err)
	}
	return path
}

// waiverBlock returns a `lint_policy:` frontmatter block waiving EXT001,
// optionally scoped. expires_at is computed at test-run time, never
// hardcoded: the CLI path evaluates waivers against the real wall clock
// (policy.Evaluate(..., time.Now())), so a literal future date would
// eventually pass and flip these tests red on a day unrelated to any
// commit.
func waiverBlock(scope []string) string {
	expiresAt := time.Now().AddDate(1, 0, 0).Format(time.RFC3339)
	var b strings.Builder
	fmt.Fprintf(&b, "lint_policy:\n  rules:\n    EXT001:\n      expires_at: %q\n      reason: \"known-good fixture for the E2E waiver guard\"\n      approved_by: \"toolchain-e2e\"\n", expiresAt)
	if len(scope) > 0 {
		b.WriteString("      scope:\n")
		for _, s := range scope {
			fmt.Fprintf(&b, "        - %q\n", s)
		}
	}
	return b.String()
}

// writeFixture writes a minimal .doclang file at <dir>/fixture.doclang, with
// or without the EXT001 waiver.
func writeFixture(t *testing.T, dir string, withWaiver bool) string {
	t.Helper()
	frontmatter := "title: \"E2E Fixture\"\n"
	if withWaiver {
		frontmatter += waiverBlock(nil)
	}
	content := fmt.Sprintf("---\n%s---\n\n# Fixture\n\nHello world.\n", frontmatter)
	path := filepath.Join(dir, "fixture.doclang")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}
	return path
}

// writeFixtureScoped writes a fixture at <dir>/<name> whose waiver's scope
// is either its own absolute path (matches) or an unrelated path (does not
// match) — isolated from writeFixture's unscoped waiver so a scope-glob
// surprise can't masquerade as a formatter regression in the round-trip
// test.
func writeFixtureScoped(t *testing.T, dir, name string, matching bool) string {
	t.Helper()
	path := filepath.Join(dir, name)
	scope := path
	if !matching {
		scope = filepath.Join(dir, "does-not-exist.doclang")
	}
	frontmatter := "title: \"E2E Scope Fixture\"\n" + waiverBlock([]string{scope})
	content := fmt.Sprintf("---\n%s---\n\n# Fixture\n\nHello world.\n", frontmatter)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write scoped fixture: %v", err)
	}
	return path
}

// resolvePolicyFromFile parses path (via the DocumentFlexParser, doclang's
// only dialect) and resolves its embedded lint_policy, for the semantic
// (never byte-for-byte) before/after comparison the fmt round-trip test
// needs — fmt re-orders and re-quotes frontmatter keys, so comparing
// resolved structs is the only comparison that means what the test claims.
func resolvePolicyFromFile(t *testing.T, path string) *linter.PolicyConfig {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	docParser := parser.NewDocumentFlexParserWithNormalization(string(content), util.NewNoop())
	doc, diags := docParser.Parse()
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("fixture has parse errors: %s", d.String())
		}
	}
	policy, err := linter.ResolvePolicyConfig("", doc.FrontMatter)
	if err != nil {
		t.Fatalf("failed to resolve lint_policy: %v", err)
	}
	if policy == nil {
		t.Fatal("expected a lint_policy in the fixture's frontmatter, got none")
	}
	return policy
}
