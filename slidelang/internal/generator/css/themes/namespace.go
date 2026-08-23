// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package themes

import (
	"regexp"
	"strings"
)

// slidelangVarPrefix is the namespace every custom-property name in a
// generated bundle must carry, so a theme's CSS never collides with the
// host page's own custom properties.
const slidelangVarPrefix = "slidelang-"

// varTokenRe matches the CSS var() function token that opens a usage —
// case-insensitively (CSS function names are ASCII case-insensitive, and
// Chromium accepts VAR(--x)/Var(--x) exactly like var(--x)) and requiring
// a word boundary immediately before it, so it can never match inside a
// longer identifier that merely ends in "var" (e.g. a hypothetical custom
// property named --my-var is never itself mistaken for a usage). No
// whitespace is allowed between "var" and its own "(" — the CSS tokenizer
// itself stops recognizing a function the moment whitespace precedes its
// "(", so "var (" is not valid CSS var() syntax to begin with, unlike
// whitespace/comments AFTER the "(" (see varInnerRe) (fifth-round finding
// on PR #223).
var varTokenRe = regexp.MustCompile(`(?i)\bvar\(`)

// varInnerRe matches the body of a var(...) call once its balanced closing
// paren has already been located by findMatchingParen — i.e. it never has
// to stop at the first ")", which is what let a nested var() inside a
// fallback (var(--a, var(--b))) go unprefixed under the old regex-only
// implementation (motor-temas-v2.md §2.1). The (?s) flag makes "." match
// newlines too: inner can legitimately contain one when a theme author
// hand-formats a wrapped fallback chain (var(--a, var(--b,\n  #fff))) —
// without it, the fallback capture group failed to reach the trailing $
// as soon as a newline sat anywhere past the leading ",", and
// FindStringSubmatch returned no match at all, which left the ENTIRE
// var(...) call — including the outer name — unprocessed by both
// namespaceVarBody and UnprefixedVarNames' walk (code-review finding on
// PR #223).
//
// The leading group captures whitespace/comment trivia between "(" and
// "--name" — var( --brand, red) and var(/* docs */--brand, red) are both
// valid CSS (whitespace and comments may sit anywhere between a
// function's "(" and its first argument), but the un-anchored "^--" left
// either form unmatched entirely, silently skipping the whole call
// (fifth-round finding on PR #223). Captured (not discarded) so
// namespaceVarBody can preserve it verbatim in the output.
var varInnerRe = regexp.MustCompile(`(?s)^((?:[ \t\r\n]|/\*.*?\*/)*)--([a-zA-Z0-9_-]+)\s*(?:,\s*(.*))?$`)

// NamespaceValue rewrites every var(--x) usage in a CSS value (or an
// arbitrary CSS fragment — it does not require a single declaration's
// worth of input) to var(--slidelang-x), preserving fallbacks and
// recursing into them so a nested var(--a, var(--b)) prefixes both names.
// It never touches custom-property *declarations* (--x: value) — that is
// NamespaceStylesheet's job, because a bare value never contains one.
// Idempotent: a name that already carries the prefix is left alone.
//
// A "var(" occurrence starting INSIDE a comment/string/url() span (see
// protectedSpanRe) is skipped instead of rewritten — content: "var(--brand)"
// is literal DISPLAYED text, not a real usage (code-review finding on
// PR #223). Deliberately does NOT fragment css at protected-span
// boundaries before scanning (an earlier version did, via
// rewriteOutsideProtectedSpans): a var(...) call can legitimately CONTAIN
// a protected span in its own fallback — var(--font, "Helvetica Neue"),
// var(--image, url("fallback.png")) are both valid CSS — and
// findMatchingParen below needs to see the whole, unsplit string to find
// the RIGHT closing paren; splitting first left it looking at a truncated
// "var(--font, " with no closing paren in sight, silently skipping the
// whole call (second-round finding on PR #223). Only the START position
// of "var(" itself is checked against protected spans — everything after
// that is scanned normally, protected spans and all.
func NamespaceValue(css string) string {
	spans := protectedSpanRe.FindAllStringIndex(css, -1)

	var out strings.Builder
	i := 0
	for {
		loc := varTokenRe.FindStringIndex(css[i:])
		if loc == nil {
			out.WriteString(css[i:])
			break
		}
		start := i + loc[0]
		openParen := i + loc[1] - 1 // index of the "(" itself, whatever case "var"/"VAR"/"Var" matched

		if insideAnySpan(start, spans) {
			out.WriteString(css[i : openParen+1])
			i = openParen + 1
			continue
		}
		out.WriteString(css[i:start])

		closeParen := findMatchingParen(css, openParen, spans)
		if closeParen == -1 {
			// Unbalanced input (truncated/invalid CSS) — emit the rest
			// verbatim rather than loop forever or panic on a slice.
			out.WriteString(css[start:])
			break
		}

		inner := css[openParen+1 : closeParen]
		out.WriteString(css[start : openParen+1]) // preserve the matched token's original casing, e.g. "VAR("
		out.WriteString(namespaceVarBody(inner))
		out.WriteString(")")
		i = closeParen + 1
	}
	return out.String()
}

// insideAnySpan reports whether pos falls within any of spans (each a
// [start, end) pair, as returned by regexp.FindAllStringIndex). Shared by
// NamespaceValue and UnprefixedVarNames to decide whether a "var(" match
// starts inside a comment/string/url() span rather than being a real
// usage.
func insideAnySpan(pos int, spans [][]int) bool {
	for _, sp := range spans {
		if pos >= sp[0] && pos < sp[1] {
			return true
		}
	}
	return false
}

// namespaceVarBody namespaces the name inside a single var(...) call's
// body (everything between the parens) and recurses into the fallback —
// via NamespaceValue, not itself — so a fallback that is plain text
// (var(--a, #fff)) is left alone and one that nests another var() call
// (var(--a, var(--b))) gets that inner call namespaced too.
func namespaceVarBody(inner string) string {
	m := varInnerRe.FindStringSubmatch(inner)
	if m == nil {
		// Not a well-formed "--name[, fallback]" body (e.g. a JS var(x)
		// unrelated to CSS custom properties slipped in) — leave as-is.
		return inner
	}
	lead, name, fallback := m[1], m[2], m[3]
	if !strings.HasPrefix(name, slidelangVarPrefix) {
		name = slidelangVarPrefix + name
	}
	if fallback == "" {
		return lead + "--" + name
	}
	return lead + "--" + name + ", " + NamespaceValue(fallback)
}

// findMatchingParen returns the index of the ")" that closes the "(" at
// openIdx, counting nesting depth so a fallback containing its own
// parenthesized calls (var(--a, rgba(0,0,0,.5))) resolves correctly. -1 if
// the input never closes.
//
// spans (as returned by protectedSpanRe.FindAllStringIndex) marks the
// comment/string/url() regions of s whose parens are NOT part of the CSS
// grammar being balanced here — without skipping them, a fallback like
// var(--token, "(") or var(--token, /* ( */ red) contains a "(" that is
// really just DISPLAYED text inside a string/comment, with no matching ")"
// of its own inside that span; counting it anyway left depth permanently
// off by one, so the scan ran to the end of s without ever seeing it return
// to 0, and NamespaceValue/UnprefixedVarNames silently gave up on the whole
// var(...) call, name included (fourth-round finding on PR #223).
func findMatchingParen(s string, openIdx int, spans [][]int) int {
	depth := 0
	for i := openIdx; i < len(s); i++ {
		if insideAnySpan(i, spans) {
			continue
		}
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// declarationRe finds custom-property *declarations* — "--name:" — as
// opposed to var(--name) *usages*. The leading alternation requires the
// declaration to start where a CSS statement can legally start (top of
// the string, right after "{" opening a rule, right after ";" closing the
// previous declaration, right after "}" closing a prior rule, or at the
// start of a line in hand-formatted CSS); that is what stops it from also
// matching the "--name" inside a var(--name) usage, which is always
// preceded by "(". Both gap groups allow spaces/tabs AND full /* ... */
// comments interleaved (zero or more of either) — one before the name,
// one between the name and the colon. Without the leading comment
// alternative, ":root { /* spacing */ --gap: 4px; }" left the DECLARATION
// unprefixed while NamespaceValue still rewrote every var(--gap) usage to
// var(--slidelang-gap) (code-review finding on PR #223). Without the
// trailing one, "--brand/* docs */: red;" — equally valid CSS — had the
// same mismatch in the other direction (fifth-round finding on PR #223).
// A bare "\n" right after a comment already re-anchors the lead-
// alternation on its own, so the leading group only matters when the
// comment sits on the same line as the declaration.
var declarationRe = regexp.MustCompile(`(^|[;{}]|\n)((?:[ \t]|(?s:/\*.*?\*/))*)--([a-zA-Z0-9_-]+)((?:[ \t]|(?s:/\*.*?\*/))*:)`)

// declConnectingCommentBeforeRe and declConnectingCommentAfterRe recognize
// when a comment (as found by protectedSpanRe) sits between a custom
// property's bare name and its colon — declarationRe's trailing gap group
// above — as opposed to a free-standing documentation comment that merely
// CONTAINS declaration-shaped text, e.g.
// "/* example: --fake: red; */ --real: blue;". Used only by
// namespaceDeclarationSpans, to decide which comments namespaceDeclarations
// must treat as transparent trivia (so a single legitimate declaration's
// name and colon are visible together in one pass) versus which must stay
// fully opaque (so a comment's own display text is never itself matched
// and rewritten as if it were a real declaration).
var declConnectingCommentBeforeRe = regexp.MustCompile(`--[a-zA-Z0-9_-]+[ \t]*$`)
var declConnectingCommentAfterRe = regexp.MustCompile(`^[ \t]*:`)

// namespaceDeclarationSpans returns the spans namespaceDeclarations must
// treat as opaque — every string and url() span from protectedSpanRe
// (always: content: "; --brand: docs" must never be mistaken for a real
// declaration, fourth-round finding on PR #223), plus every COMMENT span
// EXCEPT one immediately preceded by a bare "--name" (no colon yet) and
// immediately followed by a colon (only whitespace in between) — that one
// is trivia belonging to a single legitimate declaration
// (--brand/* docs */: red;) and must stay visible to declarationRe instead
// of being carved out and hidden from it (fifth-round finding on PR #223:
// "el scanner de declaraciones debería tratar los comentarios como trivia
// válida entre el nombre... y ':', sin perder la protección contra
// coincidencias dentro de strings/comentarios" — this satisfies both
// halves: the connecting comment stays visible, every OTHER comment stays
// opaque, so a documentation comment that happens to contain
// declaration-shaped text is never matched).
func namespaceDeclarationSpans(css string) [][]int {
	var spans [][]int
	for _, m := range protectedSpanRe.FindAllStringIndex(css, -1) {
		start, end := m[0], m[1]
		if strings.HasPrefix(css[start:end], "/*") &&
			declConnectingCommentBeforeRe.MatchString(css[:start]) &&
			declConnectingCommentAfterRe.MatchString(css[end:]) {
			continue
		}
		spans = append(spans, m)
	}
	return spans
}

// namespaceDeclarations rewrites custom-property declarations (--x: value)
// to --slidelang-x: value. Split out from NamespaceValue because a
// declaration's LHS is never itself inside a var(...) call, so it needs
// its own detection pass — done first so its output composes cleanly with
// NamespaceValue's usage rewriting in NamespaceStylesheet.
//
// Runs only outside the spans namespaceDeclarationSpans marks opaque
// (strings, url() calls, and free-standing comments): declarationRe's lead
// alternation includes a bare ";", so text like
// content: "; --brand: documentation only" was matching INSIDE the string
// literal and rewriting display text to --slidelang-brand (fourth-round
// finding on PR #223). A comment namespaceDeclarationSpans identifies as
// sitting between a bare name and its colon is deliberately NOT included
// in that opaque list — see its doc comment — so declarationRe still sees
// the name and colon together in both directions:
// ":root { /* spacing */ --gap: 4px; }" (comment before the name) and
// "--brand/* docs */: red;" (comment before the colon, fifth-round
// finding).
func namespaceDeclarations(css string) string {
	return rewriteOutsideSpanList(css, namespaceDeclarationSpans(css), func(segment string) string {
		return declarationRe.ReplaceAllStringFunc(segment, func(match string) string {
			sub := declarationRe.FindStringSubmatch(match)
			lead, ws, name, colon := sub[1], sub[2], sub[3], sub[4]
			if strings.HasPrefix(name, slidelangVarPrefix) {
				return match
			}
			return lead + ws + "--" + slidelangVarPrefix + name + colon
		})
	})
}

// NamespaceStylesheet rewrites a full CSS stylesheet — both var(--x)
// usages and --x: declarations — to the --slidelang-x form. Use this for
// any CSS that may declare its own custom properties (base CSS, and
// external-theme CSS per §2.1); use NamespaceValue alone for a single
// value that can only ever contain usages, like the values GenerateThemeCSS
// emits into its :root block.
func NamespaceStylesheet(css string) string {
	return NamespaceValue(namespaceDeclarations(css))
}

// UnprefixedVarNames returns, in first-appearance order and without
// duplicates, every var(--x) usage's name in css that does not carry the
// --slidelang- prefix — including names nested inside a fallback. Used
// by the strict theme validator (§2.1's contract: the motor validates a
// third-party theme's CSS, it does not rewrite it) to reject a styles.css
// before namespacing ever masks the mismatch a rewrite would have hidden.
func UnprefixedVarNames(css string) []string {
	seen := make(map[string]bool)
	var names []string
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}

	// A "var(" occurrence starting INSIDE a comment/string/url() span (see
	// protectedSpanRe) is skipped — content: "var(--brand)" reports
	// nothing, since "brand" there is display text, not a real usage
	// (code-review finding on PR #223, the detection half of the same bug
	// fixed in NamespaceValue). Like NamespaceValue, this deliberately
	// checks only the START position of each "var(" against the spans
	// instead of fragmenting s first — findMatchingParen needs the whole,
	// unsplit string to correctly close a var(...) call whose OWN fallback
	// legitimately contains a protected span (var(--font, "Helvetica Neue"),
	// var(--image, url("fallback.png"))); fragmenting first silently
	// missed the whole call, name included (second-round finding on
	// PR #223).
	var walk func(s string)
	walk = func(s string) {
		spans := protectedSpanRe.FindAllStringIndex(s, -1)
		i := 0
		for {
			loc := varTokenRe.FindStringIndex(s[i:])
			if loc == nil {
				return
			}
			start := i + loc[0]
			openParen := i + loc[1] - 1
			if insideAnySpan(start, spans) {
				i = openParen + 1
				continue
			}
			closeParen := findMatchingParen(s, openParen, spans)
			if closeParen == -1 {
				return
			}
			inner := s[openParen+1 : closeParen]
			if m := varInnerRe.FindStringSubmatch(inner); m != nil {
				name, fallback := m[2], m[3]
				if !strings.HasPrefix(name, slidelangVarPrefix) {
					add(name)
				}
				if fallback != "" {
					walk(fallback)
				}
			}
			i = closeParen + 1
		}
	}
	walk(css)
	return names
}

// classSelectorRe is the same detection pattern
// CSSFileLoader.ApplyNamespacing uses to rewrite the toolchain's OWN CSS —
// reused here only to detect, never to rewrite, a third-party theme's
// selectors.
var classSelectorRe = regexp.MustCompile(`\.([a-zA-Z][\w-]*)`)

// protectedSpanRe matches the CSS regions where a literal "." followed by
// letters is never a class selector: comments, quoted strings, and
// url(...) calls. Without excluding these, url("./Brand.woff2") reports
// a bogus class ".woff2" (code-review finding on PR #223) — a false
// positive that would fail --strict for exactly the @font-face CSS
// motor-temas-v2.md §2.3 is meant to enable. The comment alternative is
// scoped (?s:...) so it alone matches across newlines; the others stay
// single-line-safe on purpose (an unterminated quote/url on one line
// should not swallow the rest of the stylesheet looking for its close).
// The url() alternative's function name is matched case-insensitively —
// (?i:url) — because CSS function names are ASCII case-insensitive and
// Chromium accepts URL(...)/Url(...) exactly like url(...); without it,
// URL("./Brand.woff2") fell outside every protected span and ".woff2" was
// reported — and, via CSSFileLoader.ApplyNamespacing, actually REWRITTEN —
// as a class selector, corrupting the asset path (fifth-round finding on
// PR #223).
var protectedSpanRe = regexp.MustCompile(`(?s:/\*.*?\*/)|"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|(?i:url)\(\s*(?:"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|[^)]*)\s*\)`)

// rewriteOutsideSpanList is rewriteOutsideProtectedSpans generalized over a
// precomputed span list instead of a regex — needed when which spans count
// as "protected" depends on more context than a single regex match can
// express (see namespaceDeclarationSpans). Applies fn to every substring of
// css that falls outside a span in spans, reassembling the result with
// each span copied through byte-for-byte untouched. spans must be sorted
// by start position and non-overlapping, as returned by
// regexp.FindAllStringIndex.
func rewriteOutsideSpanList(css string, spans [][]int, fn func(string) string) string {
	var out strings.Builder
	last := 0
	for _, m := range spans {
		start, end := m[0], m[1]
		out.WriteString(fn(css[last:start]))
		out.WriteString(css[start:end])
		last = end
	}
	out.WriteString(fn(css[last:]))
	return out.String()
}

// rewriteOutsideProtectedSpans applies fn to every substring of css that
// falls outside a comment/string/url() span (see protectedSpanRe),
// reassembling the result with each protected span copied through
// byte-for-byte untouched. Shared by UnprefixedClassSelectors (fn only
// scans) and CSSFileLoader.ApplyNamespacing (fn rewrites) so an asset
// path's extension or a string literal's content is never mistaken for a
// class selector by either.
func rewriteOutsideProtectedSpans(css string, fn func(string) string) string {
	return rewriteOutsideSpanList(css, protectedSpanRe.FindAllStringIndex(css, -1), fn)
}

// RewriteOutsideProtectedSpans is the exported form of
// rewriteOutsideProtectedSpans, for callers outside this package (e.g.
// CSSFileLoader.ApplyNamespacing in the css package) that need the same
// comment/string/url() protection when rewriting class selectors.
func RewriteOutsideProtectedSpans(css string, fn func(string) string) string {
	return rewriteOutsideProtectedSpans(css, fn)
}

// KnownUnprefixedClasses lists every class name the engine's own generated
// HTML legitimately emits WITHOUT the slidelang- prefix — a historical
// inconsistency with the slidelang-* convention used everywhere else, not
// a bug in a theme's CSS for targeting them bare. A theme MUST use these
// exact bare names to match the real markup, so both
// UnprefixedClassSelectors (the strict validator) and
// CSSFileLoader.ApplyNamespacing's default ExcludeClasses (the rewriter,
// which uses this same list — see file_loader.go) treat them as already
// namespaced instead of flagging/prefixing them (code-review finding on
// PR #223: --strict rejected startup-tech's correct
// .slidelang-tab.active as if "active" needed the prefix, and the
// rewriter had separately turned the code-group's real bare <div
// class="tabs"> target into an unmatchable .slidelang-tabs).
// KnownUnprefixedClasses is verified two ways, deliberately not by reading
// template literals: TestRenderHTMLPreview_EveryElementType_BareClassesAreKnown
// (slidelang/internal/generator/bare_class_audit_test.go) renders one AST
// covering every server-rendered element type through the real pipeline
// and fails if it finds an un-prefixed class not listed here — that is
// what caught that a Round-2 fix on this PR had it backwards for "tabs"/
// "code-content" (see below). The entries below that classList.add/toggle
// at runtime (template/utilities.go, template/directives.go) can't be
// seen by that test — server-rendered HTML never contains them — so they
// stay here backed by a direct grep of that JS source instead.
var KnownUnprefixedClasses = map[string]bool{
	// Toggled by JS at runtime (classList.add/remove/toggle) — never
	// present in the initial server-rendered HTML for these two; "active"
	// IS present initially for the first tab/code-block/checklist-adjacent
	// element in some contexts.
	"active":        true, // template/utilities.go tab/details handlers, template/base.go code-group
	"checked":       true, // template/base.go's checklist checkbox <li>/<input>, from ChecklistItem.Checked
	"expanded":      true, // template/utilities.go's details/collapsible toggle
	"timer-expired": true, // template/directives.go's slide-timer directive
	"full-screen":   true, // template/directives.go's full-screen toggle
}

// KnownUnprefixedClassPrefixes is KnownUnprefixedClasses' open-ended
// counterpart: PREFIXES the engine deliberately emits bare because the
// exact suffix is unbounded, not enumerable like the fixed set above.
// "language-" is the Prism.js/highlight.js convention template/base.go
// emits on every <code class="language-{{.Language}}"> (one per
// language an author's code blocks actually use — go, python, yaml, ...,
// unbounded) — TestRenderHTMLPreview_EveryElementType_BareClassesAreKnown
// (package generator) is what surfaced this: prefixing it to
// "slidelang-language-go" would silently break syntax highlighting, since
// the client-side highlighter looks for this exact, un-namespaced
// convention.
var KnownUnprefixedClassPrefixes = []string{"language-"}

// hasKnownUnprefixedPrefix reports whether class starts with one of
// KnownUnprefixedClassPrefixes.
func hasKnownUnprefixedPrefix(class string) bool {
	for _, p := range KnownUnprefixedClassPrefixes {
		if strings.HasPrefix(class, p) {
			return true
		}
	}
	return false
}

// IsKnownUnprefixedClass reports whether class is one the engine
// deliberately emits without the slidelang- prefix — an exact
// KnownUnprefixedClasses entry or one carrying a KnownUnprefixedClassPrefixes
// prefix. The single check shared by UnprefixedClassSelectors (the strict
// validator) and CSSFileLoader.ApplyNamespacing (the rewriter, in the css
// package) so the two can never diverge on which classes are exempt.
// Before this existed, ApplyNamespacing consulted only the fixed-name half
// (via its ExcludeClasses list) and not the prefix half: a theme's
// .language-go passed `themes validate --strict` but was still rewritten to
// .slidelang-language-go when the theme actually loaded, silently breaking
// Prism.js/highlight.js syntax highlighting even though validation said the
// theme was fine (fourth-round finding on PR #223).
func IsKnownUnprefixedClass(class string) bool {
	return KnownUnprefixedClasses[class] || hasKnownUnprefixedPrefix(class)
}

// UnprefixedClassSelectors returns, in first-appearance order and without
// duplicates, every class selector in css that does not start with the
// slidelang- prefix and is not a KnownUnprefixedClasses entry. Scans only
// outside comments/strings/url() calls (see protectedSpanRe) so an asset
// filename or string content is never reported as a selector.
func UnprefixedClassSelectors(css string) []string {
	seen := make(map[string]bool)
	var classes []string
	rewriteOutsideProtectedSpans(css, func(segment string) string {
		for _, m := range classSelectorRe.FindAllStringSubmatch(segment, -1) {
			class := m[1]
			if strings.HasPrefix(class, slidelangVarPrefix) || seen[class] || IsKnownUnprefixedClass(class) {
				continue
			}
			seen[class] = true
			classes = append(classes, class)
		}
		return segment
	})
	return classes
}
