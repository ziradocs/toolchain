// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import "testing"

// minimalManifest is a theme.json that satisfies ValidateTheme's required
// variables regardless of strict mode, so these tests isolate the CSS
// namespacing checks from the unrelated manifest-completeness checks.
const minimalManifest = `{
  "name": "test-theme",
  "version": "1.0.0",
  "description": "test",
  "author": "test",
  "variables": {
    "--slidelang-primary-color": "#000",
    "--slidelang-secondary-color": "#111",
    "--slidelang-font-main": "sans-serif",
    "--slidelang-font-size-base": "1rem",
    "--slidelang-line-height-base": "1.5",
    "--slidelang-background-color": "#fff",
    "--slidelang-text-color": "#000"
  }
}`

func TestUnprefixedVarNames_FindsUsagesIncludingNested(t *testing.T) {
	css := `.slide blockquote {
    border-left: 4px solid var(--primary-color);
    background: var(--slidelang-bg-code);
    color: var(--text-on-closing, var(--bg-white));
}`
	got := UnprefixedVarNames(css)
	want := []string{"primary-color", "text-on-closing", "bg-white"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("index %d: got %q, want %q (full: %v)", i, got[i], name, got)
		}
	}
}

func TestUnprefixedClassSelectors_FindsCompoundSelectors(t *testing.T) {
	css := `.slide.title-slide, .slidelang-cover-slide { color: red; }`
	got := UnprefixedClassSelectors(css)
	want := []string{"slide", "title-slide"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("index %d: got %q, want %q (full: %v)", i, got[i], name, got)
		}
	}
}

// TestStrictValidator_RejectsUnprefixedCSS is the strict half of the §2.1
// validator contract: a styles.css using var(--x) and .slide (no
// namespace) — exactly the shape of the live modern-blue/startup-tech*
// themes — must fail `themes validate --strict`.
func TestStrictValidator_RejectsUnprefixedCSS(t *testing.T) {
	theme, err := LoadExternalThemeFromBytes([]byte(minimalManifest), []byte(
		`.slide blockquote { background: var(--bg-code); }`))
	if err != nil {
		t.Fatalf("failed to construct test theme: %v", err)
	}

	err = NewStrictThemeValidator().ValidateTheme(theme)
	if err == nil {
		t.Fatal("expected strict validation to reject unprefixed var()/class selectors")
	}
}

// TestNonStrictValidator_AcceptsUnprefixedCSS is the load-bearing
// non-regression: ThemeLoader.LoadTheme validates with the NON-strict
// validator and falls back to "default" on any error, so if this ever
// starts rejecting, modern-blue and both startup-tech themes stop loading
// entirely instead of just shipping their pre-existing broken decorative
// CSS — which is a worse outcome than the bug §2.1 fixes, and explicitly
// out of scope until §5 (retiring the seven bundled themes) happens.
func TestNonStrictValidator_AcceptsUnprefixedCSS(t *testing.T) {
	theme, err := LoadExternalThemeFromBytes([]byte(minimalManifest), []byte(
		`.slide blockquote { background: var(--bg-code); }`))
	if err != nil {
		t.Fatalf("failed to construct test theme: %v", err)
	}

	if err := NewThemeValidator().ValidateTheme(theme); err != nil {
		t.Fatalf("expected non-strict validation to accept unprefixed CSS (regression risk: this is what today's shipped themes look like), got: %v", err)
	}
}

// TestStrictValidator_AcceptsProperlyNamespacedCSS confirms the strict
// checks aren't simply always-fail: a theme that already follows the
// contract (elegant-minimal's shape) passes.
func TestStrictValidator_AcceptsProperlyNamespacedCSS(t *testing.T) {
	theme, err := LoadExternalThemeFromBytes([]byte(minimalManifest), []byte(
		`.slidelang-slide.slidelang-title-slide { background: var(--slidelang-bg-title-slide); }`))
	if err != nil {
		t.Fatalf("failed to construct test theme: %v", err)
	}

	if err := NewStrictThemeValidator().ValidateTheme(theme); err != nil {
		t.Errorf("expected strict validation to accept fully-namespaced CSS, got: %v", err)
	}
}
