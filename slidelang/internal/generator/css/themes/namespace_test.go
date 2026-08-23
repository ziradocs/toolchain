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
