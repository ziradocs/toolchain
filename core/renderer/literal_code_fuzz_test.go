// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"regexp"
	"strings"
	"testing"
)

var codeContentPattern = regexp.MustCompile(`(?s)<code[^>]*>(.*?)</code>`)

func codeContents(html string) []string {
	var out []string
	for _, m := range codeContentPattern.FindAllStringSubmatch(html, -1) {
		out = append(out, m[1])
	}
	return out
}

// FuzzLiteralCodeProse es diferencial: con la opción de código literal, el
// contenido de cada <code> del render es el mismo que sale renderizando el
// texto sin sustituir nada. Si proseVariables y el renderer emparejan los code
// spans distinto (entre líneas, por ejemplo), una {{variable}} dentro de un
// <code> sale sustituida y la prueba falla.
func FuzzLiteralCodeProse(f *testing.F) {
	for _, seed := range []string{
		"a {{x}} `b {{x}}` c",
		"x ` y\n`{{x}}`",
		"`uno\n{{x}}` dos",
		"``a ` {{x}}`` {{x}}",
		"texto {{x}}\n```yaml\na: {{x}}\n```\n`{{x}}` y {{x}}",
		"- item `{{x}}`\n- otro {{x}}",
		"\\`{{x}}` `{{x}}\\`",
		"```\n{{x}}",
	} {
		f.Add(seed)
	}
	vars := map[string]interface{}{"x": "VAL"}
	f.Fuzz(func(t *testing.T, text string) {
		literal := ProcessTextWithVariablesAndMarkdownSecure(text, withLiteralCode(vars))
		plain := ProcessTextWithVariablesAndMarkdownSecure(text, nil)
		got, want := codeContents(literal), codeContents(plain)
		if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("code contents differ for %q:\n literal %q\n   plain %q", text, got, want)
		}
		lp := renderProseWithLists(proseVariables(text, withLiteralCode(vars)), true)
		pp := renderProseWithLists(text, true)
		if got, want := codeContents(lp), codeContents(pp); strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			t.Fatalf("prose-with-lists code contents differ for %q:\n literal %q\n   plain %q", text, got, want)
		}
	})
}
