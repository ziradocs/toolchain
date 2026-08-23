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

// varInnerRe matches the body of a var(...) call once its balanced closing
// paren has already been located by findMatchingParen — i.e. it never has
// to stop at the first ")", which is what let a nested var() inside a
// fallback (var(--a, var(--b))) go unprefixed under the old regex-only
// implementation (motor-temas-v2.md §2.1).
var varInnerRe = regexp.MustCompile(`^--([a-zA-Z0-9_-]+)\s*(?:,\s*(.*))?$`)

// NamespaceValue rewrites every var(--x) usage in a CSS value (or an
// arbitrary CSS fragment — it does not require a single declaration's
// worth of input) to var(--slidelang-x), preserving fallbacks and
// recursing into them so a nested var(--a, var(--b)) prefixes both names.
// It never touches custom-property *declarations* (--x: value) — that is
// NamespaceStylesheet's job, because a bare value never contains one.
// Idempotent: a name that already carries the prefix is left alone.
func NamespaceValue(css string) string {
	var out strings.Builder
	i := 0
	for {
		idx := strings.Index(css[i:], "var(")
		if idx == -1 {
			out.WriteString(css[i:])
			break
		}
		start := i + idx
		out.WriteString(css[i:start])

		openParen := start + len("var") // index of the "(" itself
		closeParen := findMatchingParen(css, openParen)
		if closeParen == -1 {
			// Unbalanced input (truncated/invalid CSS) — emit the rest
			// verbatim rather than loop forever or panic on a slice.
			out.WriteString(css[start:])
			break
		}

		inner := css[openParen+1 : closeParen]
		out.WriteString("var(")
		out.WriteString(namespaceVarBody(inner))
		out.WriteString(")")
		i = closeParen + 1
	}
	return out.String()
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
	name, fallback := m[1], m[2]
	if !strings.HasPrefix(name, slidelangVarPrefix) {
		name = slidelangVarPrefix + name
	}
	if fallback == "" {
		return "--" + name
	}
	return "--" + name + ", " + NamespaceValue(fallback)
}

// findMatchingParen returns the index of the ")" that closes the "(" at
// openIdx, counting nesting depth so a fallback containing its own
// parenthesized calls (var(--a, rgba(0,0,0,.5))) resolves correctly. -1 if
// the input never closes.
func findMatchingParen(s string, openIdx int) int {
	depth := 0
	for i := openIdx; i < len(s); i++ {
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
// preceded by "(".
var declarationRe = regexp.MustCompile(`(^|[;{}]|\n)([ \t]*)--([a-zA-Z0-9_-]+)(\s*:)`)

// namespaceDeclarations rewrites custom-property declarations (--x: value)
// to --slidelang-x: value. Split out from NamespaceValue because a
// declaration's LHS is never itself inside a var(...) call, so it needs
// its own detection pass — done first so its output composes cleanly with
// NamespaceValue's usage rewriting in NamespaceStylesheet.
func namespaceDeclarations(css string) string {
	return declarationRe.ReplaceAllStringFunc(css, func(match string) string {
		sub := declarationRe.FindStringSubmatch(match)
		lead, ws, name, colon := sub[1], sub[2], sub[3], sub[4]
		if strings.HasPrefix(name, slidelangVarPrefix) {
			return match
		}
		return lead + ws + "--" + slidelangVarPrefix + name + colon
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

	var walk func(s string)
	walk = func(s string) {
		i := 0
		for {
			idx := strings.Index(s[i:], "var(")
			if idx == -1 {
				return
			}
			start := i + idx
			openParen := start + len("var")
			closeParen := findMatchingParen(s, openParen)
			if closeParen == -1 {
				return
			}
			inner := s[openParen+1 : closeParen]
			if m := varInnerRe.FindStringSubmatch(inner); m != nil {
				name, fallback := m[1], m[2]
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

// UnprefixedClassSelectors returns, in first-appearance order and without
// duplicates, every class selector in css that does not start with the
// slidelang- prefix.
func UnprefixedClassSelectors(css string) []string {
	seen := make(map[string]bool)
	var classes []string
	for _, m := range classSelectorRe.FindAllStringSubmatch(css, -1) {
		class := m[1]
		if strings.HasPrefix(class, slidelangVarPrefix) || seen[class] {
			continue
		}
		seen[class] = true
		classes = append(classes, class)
	}
	return classes
}
