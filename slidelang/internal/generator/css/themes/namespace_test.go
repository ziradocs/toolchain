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
// ExcludeClasses) for exactly this. ".tabs"/".code-content" are NOT
// examples here (a third-round finding on this PR: those two ARE
// auto-prefixed by namespaceTemplateClasses, unlike "active" — see
// KnownUnprefixedClasses's doc comment and
// TestRenderHTMLPreview_EveryElementType_BareClassesAreKnown in package
// generator, which verifies the full list against real rendered output).
func TestUnprefixedClassSelectors_AcceptsKnownUnprefixedClasses(t *testing.T) {
	css := `.slidelang-tab.active { color: red; }
.slidelang-checklist-item.checked { color: green; }`
	got := UnprefixedClassSelectors(css)
	if len(got) != 0 {
		t.Errorf("UnprefixedClassSelectors(%q) = %v, want none — active/checked are legitimately bare in the engine's own markup", css, got)
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

// TestNamespaceValue_VarWithQuotedFallback and
// TestUnprefixedVarNames_VarWithQuotedFallback are the third-round PR #223
// finding on the fix above: the FIRST version of the string/url()
// protection fragmented css at protected-span boundaries BEFORE scanning
// for var(...) calls, which broke a var(...) call whose OWN fallback
// legitimately contains a string or url() — both valid, common CSS
// (font-family: var(--font, "Helvetica Neue") needs the quotes because the
// font name has a space). The fragmented scanner received only
// "var(--font, " with no closing paren in sight and silently gave up on
// the WHOLE call, name included. The fix checks only whether a "var("
// match's START position is inside a protected span, without ever
// splitting the string, so findMatchingParen still sees the whole,
// correctly-balanced call.
func TestNamespaceValue_VarWithQuotedFallback(t *testing.T) {
	cases := []struct {
		name string
		css  string
		want string
	}{
		{
			"quoted font name fallback",
			`font-family: var(--font, "Helvetica Neue");`,
			`font-family: var(--slidelang-font, "Helvetica Neue");`,
		},
		{
			"url() with a quoted path fallback",
			`background: var(--image, url("fallback.png"));`,
			`background: var(--slidelang-image, url("fallback.png"));`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NamespaceValue(c.css); got != c.want {
				t.Errorf("NamespaceValue(%q) = %q, want %q", c.css, got, c.want)
			}
		})
	}
}

func TestUnprefixedVarNames_VarWithQuotedFallback(t *testing.T) {
	cases := []struct {
		name string
		css  string
		want string
	}{
		{"quoted font name fallback", `font-family: var(--font, "Helvetica Neue");`, "font"},
		{"url() with a quoted path fallback", `background: var(--image, url("fallback.png"));`, "image"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := UnprefixedVarNames(c.css)
			if len(got) != 1 || got[0] != c.want {
				t.Errorf("UnprefixedVarNames(%q) = %v, want [%q]", c.css, got, c.want)
			}
		})
	}
}

// TestNamespaceValue_ParenInsideProtectedSpanDoesNotUnbalance and
// TestUnprefixedVarNames_ParenInsideProtectedSpanDoesNotUnbalance are the
// fourth-round PR #223 finding: findMatchingParen counted every "(" and ")"
// in the input regardless of whether they sat inside a comment/string/
// url() span. A fallback whose protected content itself contains an
// unmatched paren — var(--token, "("), var(--token, /* ( */ red),
// var(--token, url("fallback(image.png")) — threw the depth count
// permanently off by one, so the scan reached the end of the string
// without returning to depth 0 and NamespaceValue/UnprefixedVarNames
// silently gave up on the whole var(...) call, name included. The fix
// skips any position inside a protected span while counting parens.
func TestNamespaceValue_ParenInsideProtectedSpanDoesNotUnbalance(t *testing.T) {
	cases := []struct {
		name string
		css  string
		want string
	}{
		{
			"unmatched paren inside a quoted fallback",
			`content: var(--token, "(");`,
			`content: var(--slidelang-token, "(");`,
		},
		{
			"unmatched paren inside a comment before the fallback value",
			`color: var(--token, /* ( */ red);`,
			`color: var(--slidelang-token, /* ( */ red);`,
		},
		{
			"unmatched paren inside a url() path",
			`background: var(--token, url("fallback(image.png"));`,
			`background: var(--slidelang-token, url("fallback(image.png"));`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NamespaceValue(c.css); got != c.want {
				t.Errorf("NamespaceValue(%q) = %q, want %q", c.css, got, c.want)
			}
		})
	}
}

func TestUnprefixedVarNames_ParenInsideProtectedSpanDoesNotUnbalance(t *testing.T) {
	cases := []struct {
		name string
		css  string
	}{
		{"unmatched paren inside a quoted fallback", `content: var(--token, "(");`},
		{"unmatched paren inside a comment before the fallback value", `color: var(--token, /* ( */ red);`},
		{"unmatched paren inside a url() path", `background: var(--token, url("fallback(image.png"));`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := UnprefixedVarNames(c.css)
			if len(got) != 1 || got[0] != "token" {
				t.Errorf("UnprefixedVarNames(%q) = %v, want [\"token\"]", c.css, got)
			}
		})
	}
}

// TestNamespaceDeclarations_IgnoresSemicolonInsideString is the fourth-round
// PR #223 finding: declarationRe's lead alternation accepts a bare ";", and
// namespaceDeclarations ran on the raw CSS with no protected-span
// awareness — so a string literal containing "; --name:" (display text,
// not a real declaration) was matched and rewritten anyway.
func TestNamespaceDeclarations_IgnoresSemicolonInsideString(t *testing.T) {
	css := `.slidelang-x::after { content: "; --brand: documentation only"; }`
	got := namespaceDeclarations(css)
	if got != css {
		t.Errorf("namespaceDeclarations rewrote text inside a string literal: got %q, want unchanged %q", got, css)
	}
}

// TestNamespaceDeclarations_StillHandlesCommentBeforeDeclaration guards
// against a regression from fixing the finding above: routing
// namespaceDeclarations through rewriteOutsideProtectedSpans must not lose
// support for a real declaration preceded by a same-line comment (already
// covered end-to-end by
// TestNamespaceStylesheet_DeclarationAfterInlineComment) — here isolated to
// namespaceDeclarations itself, since the fix changes how that comment
// case reaches a match (the segment starting right after the excised
// comment span, anchored by declarationRe's own "^" alternative, rather
// than the comment being swallowed by the regex's gap group in one pass).
func TestNamespaceDeclarations_StillHandlesCommentBeforeDeclaration(t *testing.T) {
	css := ":root { /* spacing */ --gap: 4px; }"
	got := namespaceDeclarations(css)
	want := ":root { /* spacing */ --slidelang-gap: 4px; }"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestNamespaceValue_CaseInsensitiveVarToken and
// TestUnprefixedVarNames_CaseInsensitiveVarToken are the fifth-round PR
// #223 finding: CSS function names are ASCII case-insensitive, and
// Chromium resolves VAR(--x) exactly like var(--x), but the scanner
// searched for the literal lowercase substring "var(" — so VAR(--brand,
// red) was left completely untouched by both the rewriter and the strict
// detector, silently leaving --brand unresolved even after its own
// declaration was renamed to --slidelang-brand.
func TestNamespaceValue_CaseInsensitiveVarToken(t *testing.T) {
	got := NamespaceValue("color: VAR(--brand, red);")
	want := "color: VAR(--slidelang-brand, red);"
	if got != want {
		t.Errorf("got %q, want %q — original casing should be preserved, only the name prefixed", got, want)
	}
}

func TestUnprefixedVarNames_CaseInsensitiveVarToken(t *testing.T) {
	got := UnprefixedVarNames("color: VAR(--brand, red);")
	if len(got) != 1 || got[0] != "brand" {
		t.Errorf("UnprefixedVarNames(%q) = %v, want [\"brand\"]", "color: VAR(--brand, red);", got)
	}
}

// TestNamespaceValue_WhitespaceAndCommentBeforeVarName and
// TestUnprefixedVarNames_WhitespaceAndCommentBeforeVarName are the other
// half of the same finding: whitespace or a comment may legally sit
// between a function's "(" and its first argument — var( --brand, red)
// and var(/* docs */--brand, red) are both valid CSS — but varInnerRe's
// un-anchored "^--" required "--name" to start immediately at position 0
// of the captured body, so either form was left completely unprocessed.
func TestNamespaceValue_WhitespaceAndCommentBeforeVarName(t *testing.T) {
	cases := []struct {
		name string
		css  string
		want string
	}{
		{"leading whitespace", "var( --brand, red)", "var( --slidelang-brand, red)"},
		{"leading comment", "var(/* docs */--brand, red)", "var(/* docs */--slidelang-brand, red)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NamespaceValue(c.css); got != c.want {
				t.Errorf("NamespaceValue(%q) = %q, want %q", c.css, got, c.want)
			}
		})
	}
}

func TestUnprefixedVarNames_WhitespaceAndCommentBeforeVarName(t *testing.T) {
	cases := []string{"var( --brand, red)", "var(/* docs */--brand, red)"}
	for _, css := range cases {
		t.Run(css, func(t *testing.T) {
			got := UnprefixedVarNames(css)
			if len(got) != 1 || got[0] != "brand" {
				t.Errorf("UnprefixedVarNames(%q) = %v, want [\"brand\"]", css, got)
			}
		})
	}
}

// TestNamespaceDeclarations_CommentBetweenNameAndColon is the fifth-round
// PR #223 finding: a comment may legally sit between a custom property's
// name and its colon (--brand/* docs */: red;, valid CSS Chromium
// resolves correctly), but declarationRe only allowed whitespace there,
// and namespaceDeclarations (as of the fourth-round fix) additionally
// carved every comment out as an opaque span before scanning — splitting
// "--brand" and ":" into two different segments that could never match
// together. The declaration was left completely unprefixed while
// NamespaceValue still rewrote every var(--brand) usage elsewhere,
// breaking the rule.
func TestNamespaceDeclarations_CommentBetweenNameAndColon(t *testing.T) {
	css := "--brand/* docs */: red;"
	got := namespaceDeclarations(css)
	want := "--slidelang-brand/* docs */: red;"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestNamespaceStylesheet_CommentBetweenNameAndColon is the end-to-end
// version: the declaration and a usage elsewhere must resolve to the same
// namespaced name.
func TestNamespaceStylesheet_CommentBetweenNameAndColon(t *testing.T) {
	css := ":root {\n    --brand/* docs */: red;\n}\n.slidelang-box { color: var(--brand); }"
	got := NamespaceStylesheet(css)
	if !strings.Contains(got, "--slidelang-brand/* docs */: red;") {
		t.Errorf("declaration with a comment before its colon was not namespaced, got:\n%s", got)
	}
	if !strings.Contains(got, "var(--slidelang-brand)") {
		t.Errorf("expected the usage to stay namespaced too, got:\n%s", got)
	}
}

// TestNamespaceDeclarations_DoesNotExposeFreeStandingCommentContent guards
// the OTHER half of the fifth-round finding's own request ("sin perder la
// protección contra coincidencias dentro de... comentarios"): a
// free-standing documentation comment that merely CONTAINS
// declaration-shaped text — not sitting between a real name and a real
// colon — must stay fully opaque, or its display text would itself get
// matched and rewritten. This is the adversarial case a naive "just stop
// treating all comments as protected spans" fix would have reintroduced:
// the comment here contains a brace and a colon of its own
// (/* config: { --fake: red } */), which — without namespaceDeclarationSpans'
// before/after check — a lead-character scan starting at the "{" INSIDE
// the comment would incorrectly treat as a valid declaration start.
func TestNamespaceDeclarations_DoesNotExposeFreeStandingCommentContent(t *testing.T) {
	css := "/* config: { --fake: red } */ --real: blue;"
	got := namespaceDeclarations(css)
	want := "/* config: { --fake: red } */ --slidelang-real: blue;"
	if got != want {
		t.Errorf("got %q, want %q — the comment's own content must stay untouched, only --real should be namespaced", got, want)
	}
}

// TestUnprefixedClassSelectors_IgnoresUppercaseURL and
// TestApplyNamespacing_IgnoresUppercaseURL are the fifth-round PR #223
// finding: CSS function names are ASCII case-insensitive and Chromium
// accepts URL(...) exactly like url(...), but protectedSpanRe's url()
// alternative matched only the literal lowercase "url(" — so
// URL("./Brand.woff2") fell outside every protected span, and ".woff2"
// was reported as (and, in ApplyNamespacing, actually rewritten into) a
// bogus class selector, corrupting the asset path.
func TestUnprefixedClassSelectors_IgnoresUppercaseURL(t *testing.T) {
	css := `.slidelang-x { background: URL(./Brand.woff2); }`
	got := UnprefixedClassSelectors(css)
	for _, c := range got {
		if strings.Contains(c, "woff2") {
			t.Errorf("UnprefixedClassSelectors reported %q from inside an uppercase URL(), got %v", c, got)
		}
	}
}

// TestNamespaceValue_DoesNotMatchInsideLongerIdentifier and
// TestUnprefixedVarNames_DoesNotMatchInsideLongerIdentifier are the
// sixth-round PR #223 finding: Go's regexp \b treats "-" as a
// non-word character, so \bvar\( still matched inside foo-var(...) — a
// token stream Chromium keeps completely literal, since that "var(" is
// not the CSS var() function at all, just the tail of some other
// identifier/function name. Without the fix, --value: foo-var(--brand);
// had --brand rewritten to --slidelang-brand even though no real var()
// usage is present.
func TestNamespaceValue_DoesNotMatchInsideLongerIdentifier(t *testing.T) {
	css := "--value: foo-var(--brand);"
	got := NamespaceValue(css)
	if got != css {
		t.Errorf("NamespaceValue(%q) = %q, want unchanged — foo-var(...) is not the CSS var() function", css, got)
	}
}

func TestUnprefixedVarNames_DoesNotMatchInsideLongerIdentifier(t *testing.T) {
	css := "--value: foo-var(--brand);"
	if got := UnprefixedVarNames(css); len(got) != 0 {
		t.Errorf("UnprefixedVarNames(%q) = %v, want none — foo-var(...) is not the CSS var() function", css, got)
	}
}

// TestNamespaceValue_CommentAfterVarName and
// TestUnprefixedVarNames_CommentAfterVarName are the sixth-round PR #223
// finding: a comment is valid CSS trivia between a var() call's name and
// whatever follows it (a comma introducing a fallback, or the closing
// paren) — Chromium resolves both var(--brand/* docs */, red) and
// var(--brand/* docs */) correctly — but varInnerRe only allowed \s*
// there, so both forms were left completely unprocessed.
func TestNamespaceValue_CommentAfterVarName(t *testing.T) {
	cases := []struct {
		name string
		css  string
		want string
	}{
		{"with fallback", "color: var(--brand/* docs */, red);", "color: var(--slidelang-brand/* docs */, red);"},
		{"without fallback", "color: var(--brand/* docs */);", "color: var(--slidelang-brand/* docs */);"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NamespaceValue(c.css); got != c.want {
				t.Errorf("NamespaceValue(%q) = %q, want %q", c.css, got, c.want)
			}
		})
	}
}

func TestUnprefixedVarNames_CommentAfterVarName(t *testing.T) {
	cases := []string{
		"color: var(--brand/* docs */, red);",
		"color: var(--brand/* docs */);",
	}
	for _, css := range cases {
		t.Run(css, func(t *testing.T) {
			got := UnprefixedVarNames(css)
			if len(got) != 1 || got[0] != "brand" {
				t.Errorf("UnprefixedVarNames(%q) = %v, want [\"brand\"]", css, got)
			}
		})
	}
}

// TestNamespaceDeclarations_NewlineBeforeColon is the sixth-round PR #223
// finding: Chromium accepts a bare newline (or carriage return / form
// feed) between a custom property's name and its colon, exactly like it
// accepts a comment there, but declarationRe's trailing gap group only
// listed "[ \t]" as whitespace — so "--brand\n: red;" was left unprefixed
// while var(--brand) usages elsewhere still got rewritten.
func TestNamespaceDeclarations_NewlineBeforeColon(t *testing.T) {
	css := "--brand\n: red;"
	got := namespaceDeclarations(css)
	want := "--slidelang-brand\n: red;"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestNamespaceDeclarations_MultipleConsecutiveCommentsBeforeColon is the
// sixth-round PR #223 finding, and a regression the fifth-round's own fix
// introduced: namespaceDeclarationSpans used to classify each comment span
// individually (is THIS comment immediately preceded by the bare name AND
// immediately followed by the colon?), which correctly handled ONE
// connecting comment but missed a RUN of several — in
// --brand/* a *//* b */: red;, the first comment is followed by another
// comment (not the colon) and the second comment is preceded by another
// comment (not the bare name), so neither qualified individually even
// though together they connect name to colon exactly like one comment
// would. Fixed by recognizing the whole trivia run as one unit
// (declarationTrailingTriviaRe) instead of judging each comment span in
// isolation.
func TestNamespaceDeclarations_MultipleConsecutiveCommentsBeforeColon(t *testing.T) {
	css := "--brand/* a *//* b */: red;"
	got := namespaceDeclarations(css)
	want := "--slidelang-brand/* a *//* b */: red;"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
