// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import (
	"strings"
	"testing"
)

func TestNamespaceValue_SimpleUsage(t *testing.T) {
	got := NamespaceValue("var(--bg-code)")
	want := "var(--slidelang-bg-code)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNamespaceValue_AlreadyNamespacedIsIdempotent(t *testing.T) {
	got := NamespaceValue("var(--slidelang-bg-code)")
	want := "var(--slidelang-bg-code)"
	if got != want {
		t.Errorf("got %q, want %q — a second pass must not double-prefix", got, want)
	}
}

func TestNamespaceValue_PlainFallbackPreserved(t *testing.T) {
	got := NamespaceValue("var(--border-color, #ddd)")
	want := "var(--slidelang-border-color, #ddd)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestNamespaceValue_NestedVarInFallback is the regression for the bug
// verified live at assets/css/base/slides.css:67
// (`color: var(--text-on-closing, var(--bg-white));`): the old
// regex-only implementation matched up to the FIRST ")", which is the
// inner var()'s close — so the inner name never got its own turn through
// the replacer and stayed unprefixed forever, even across repeated
// passes, because the outer name (already prefixed) short-circuited the
// `changed` flag before the inner one was ever visited standalone.
func TestNamespaceValue_NestedVarInFallback(t *testing.T) {
	got := NamespaceValue("var(--text-on-closing, var(--bg-white))")
	want := "var(--slidelang-text-on-closing, var(--slidelang-bg-white))"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNamespaceValue_DoublyNestedFallback(t *testing.T) {
	got := NamespaceValue("var(--a, var(--b, var(--c)))")
	want := "var(--slidelang-a, var(--slidelang-b, var(--slidelang-c)))"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNamespaceValue_FallbackWithParensNotVar(t *testing.T) {
	// A fallback that contains a parenthesized call other than var() —
	// e.g. rgba(...) — must not confuse the balanced-paren matcher into
	// stopping early or leaving stray parens behind.
	got := NamespaceValue("var(--overlay, rgba(0, 0, 0, 0.5))")
	want := "var(--slidelang-overlay, rgba(0, 0, 0, 0.5))"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNamespaceValue_MultipleUsagesInOneValue(t *testing.T) {
	got := NamespaceValue("linear-gradient(135deg, var(--primary-color), var(--accent-color))")
	want := "linear-gradient(135deg, var(--slidelang-primary-color), var(--slidelang-accent-color))"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNamespaceDeclarations_TopLevelAndAfterSemicolon(t *testing.T) {
	css := "--foo: red; color: blue; --bar: green;"
	got := namespaceDeclarations(css)
	want := "--slidelang-foo: red; color: blue; --slidelang-bar: green;"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNamespaceDeclarations_DoesNotTouchUsages(t *testing.T) {
	// A var(--x) usage must never be mistaken for a declaration — it is
	// preceded by "(", not by ";"/"{"/a newline/start-of-string.
	css := ".foo { color: var(--text-color); }"
	got := namespaceDeclarations(css)
	if got != css {
		t.Errorf("namespaceDeclarations must leave usages untouched, got %q", got)
	}
}

func TestNamespaceDeclarations_AlreadyPrefixedIdempotent(t *testing.T) {
	css := "--slidelang-foo: red;"
	got := namespaceDeclarations(css)
	if got != css {
		t.Errorf("got %q, want unchanged %q", got, css)
	}
}

// TestNamespaceStylesheet_UsageAndDeclarationTogether is the shape a
// third-party theme's styles.css could legally use: a local helper
// declared with :root { --x: ... } and then referenced with var(--x) —
// both must resolve to the same namespaced name for the CSS to still
// work after namespacing (§2.1 decision: declarations are namespaced,
// not just usages).
func TestNamespaceStylesheet_UsageAndDeclarationTogether(t *testing.T) {
	css := ":root { --helper: 4px; } .box { border-radius: var(--helper); }"
	got := NamespaceStylesheet(css)
	want := ":root { --slidelang-helper: 4px; } .box { border-radius: var(--slidelang-helper); }"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestNamespaceStylesheet_ModernBlueBlockquoteRegression is the exact bug
// documented in docs/developer/motor-temas-v2.md §2.1 and reproduced
// live in slidelang/themes/modern-blue/styles.css:154-160: an external
// theme's blockquote rule referenced --primary-color/--secondary-color/
// --bg-code without the prefix, so none of the three resolved against the
// --slidelang-* variables the theme's own :root block (from theme.json)
// actually emits.
func TestNamespaceStylesheet_ModernBlueBlockquoteRegression(t *testing.T) {
	css := `.slide blockquote {
    border-left: 4px solid var(--primary-color);
    color: var(--secondary-color);
    background: var(--bg-code);
}`
	got := NamespaceStylesheet(css)
	for _, want := range []string{
		"var(--slidelang-primary-color)",
		"var(--slidelang-secondary-color)",
		"var(--slidelang-bg-code)",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected namespaced output to contain %q, got:\n%s", want, got)
		}
	}
}

// TestNamespaceValue_MultilineFallback is a code-review finding on PR #223:
// varInnerRe used a bare (.*)$, and Go's "." does not match "\n" without
// the (?s) flag — so a hand-formatted fallback chain that wraps mid-value
// (the outer var's fallback itself contains a var() whose OWN fallback is
// on the next line) failed FindStringSubmatch entirely, leaving the whole
// var(...) call — including the OUTER name — unprocessed. A newline
// immediately after the leading comma was already safe (\s* absorbs it);
// this covers the case that wasn't.
func TestNamespaceValue_MultilineFallback(t *testing.T) {
	css := "var(--a, var(--b,\n  #fff))"
	got := NamespaceValue(css)
	for _, want := range []string{"--slidelang-a", "--slidelang-b"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output, got %q", want, got)
		}
	}
}

// TestUnprefixedVarNames_MultilineFallback is the UnprefixedVarNames half
// of the same finding: the strict validator must still detect an
// unprefixed name inside a multiline fallback, not just NamespaceValue's
// rewriter.
func TestUnprefixedVarNames_MultilineFallback(t *testing.T) {
	css := "var(--a, var(--b,\n  #fff))"
	names := UnprefixedVarNames(css)
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found["a"] || !found["b"] {
		t.Errorf("UnprefixedVarNames(%q) = %v, want both \"a\" and \"b\"", css, names)
	}
}

// TestUnprefixedClassSelectors_IgnoresAssetURLs is the P1 code-review
// finding on PR #223: classSelectorRe matched ANY "." followed by letters
// anywhere in the stylesheet, so url("./Brand.woff2") reported a bogus
// class ".woff2" — which would fail --strict for exactly the @font-face
// CSS motor-temas-v2.md §2.3 (self-hosted fonts) is meant to enable.
func TestUnprefixedClassSelectors_IgnoresAssetURLs(t *testing.T) {
	css := `@font-face {
  font-family: 'Brand';
  src: url("./Brand.woff2") format("woff2"), url('./Brand.woff') format('woff');
}
.slidelang-real-class { color: red; }`
	got := UnprefixedClassSelectors(css)
	for _, unwanted := range []string{"woff2", "woff", "Brand"} {
		for _, c := range got {
			if c == unwanted {
				t.Errorf("UnprefixedClassSelectors reported %q as a class selector (from inside url()/a string), got %v", unwanted, got)
			}
		}
	}
}

// TestUnprefixedClassSelectors_IgnoresQuotedContentAndComments covers the
// other two protected-span cases: a string literal (content:) and a
// comment, either of which can legally contain "." followed by letters
// with no selector meaning at all.
func TestUnprefixedClassSelectors_IgnoresQuotedContentAndComments(t *testing.T) {
	css := `/* see .not-a-selector.example for context */
.icon::after { content: ".not-a-selector-either"; }
.slidelang-real { color: blue; }`
	got := UnprefixedClassSelectors(css)
	for _, c := range got {
		if strings.Contains(c, "not-a-selector") {
			t.Errorf("UnprefixedClassSelectors reported %q from inside a comment/string, got %v", c, got)
		}
	}
	found := false
	for _, c := range got {
		if c == "icon" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected the real .icon selector to still be detected, got %v", got)
	}
}

// TestUnprefixedClassSelectors_AcceptsKnownUnprefixedClasses is the second
// PR #223 review round's finding: --strict rejected startup-tech's
// .slidelang-tab.active as if "active" needed the slidelang- prefix, even
// though template/base.go's code-group toggles that exact bare class at
// runtime (classList.add('active')) — a theme has no way to target the
// real markup other than by using the bare name. KnownUnprefixedClasses
// is the shared allowlist (also used by CSSFileLoader's default
// ExcludeClasses) for exactly this.
func TestUnprefixedClassSelectors_AcceptsKnownUnprefixedClasses(t *testing.T) {
	css := `.slidelang-tab.active { color: red; }
.slidelang-element.slidelang-code-group .tabs { display: flex; }
.slidelang-element.slidelang-code-group .code-content { padding: 1rem; }`
	got := UnprefixedClassSelectors(css)
	if len(got) != 0 {
		t.Errorf("UnprefixedClassSelectors(%q) = %v, want none — active/tabs/code-content are legitimately bare in the engine's own markup", css, got)
	}
}

// TestNamespaceStylesheet_DeclarationAfterInlineComment is a second-round
// PR #223 finding: declarationRe's gap group only allowed spaces/tabs
// between the lead character ("{"/";"/"}"/newline) and "--name", so a
// same-line comment between them (":root { /* spacing */ --gap: 4px; }")
// left the DECLARATION unprefixed while NamespaceValue still rewrote every
// var(--gap) usage — a mismatch that breaks the rule.
func TestNamespaceStylesheet_DeclarationAfterInlineComment(t *testing.T) {
	css := `:root { /* spacing */ --gap: 4px; }
.slidelang-box { padding: var(--gap); }`
	got := NamespaceStylesheet(css)
	if !strings.Contains(got, "--slidelang-gap: 4px") {
		t.Errorf("declaration after an inline comment was not namespaced, got:\n%s", got)
	}
	if !strings.Contains(got, "var(--slidelang-gap)") {
		t.Errorf("expected the usage to stay namespaced too, got:\n%s", got)
	}
	if strings.Contains(got, "--gap:") {
		t.Errorf("found a leftover un-namespaced declaration, got:\n%s", got)
	}
}

// TestNamespaceValue_IgnoresStringContent and
// TestUnprefixedVarNames_IgnoresStringContent are the second-round PR #223
// finding that protectedSpanRe (added for UnprefixedClassSelectors) was
// never applied to the var()-usage side: content: "var(--brand)" is
// literal DISPLAYED text, not a real custom-property usage, but both the
// rewriter and the detector treated it as one.
func TestNamespaceValue_IgnoresStringContent(t *testing.T) {
	css := `.icon::after { content: "var(--brand)"; }`
	got := NamespaceValue(css)
	if got != css {
		t.Errorf("NamespaceValue rewrote text inside a string literal: got %q, want unchanged %q", got, css)
	}
}

func TestUnprefixedVarNames_IgnoresStringContent(t *testing.T) {
	css := `.icon::after { content: "var(--brand)"; }`
	if got := UnprefixedVarNames(css); len(got) != 0 {
		t.Errorf("UnprefixedVarNames(%q) = %v, want none — \"brand\" is inside a string literal, not a real usage", css, got)
	}
}
