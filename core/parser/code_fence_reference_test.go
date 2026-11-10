// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/util"
)

// fenceTokens son las líneas con las que FuzzFenceReadingIsReported arma
// documentos: cercas de tres y cuatro backticks, con y sin info string, con
// un backtick en el info string, prosa, encabezados, listas y bloques.
var fenceTokens = []string{
	"```", "````", "```js", "````md", "```js`", "```ts title=`a.ts`", "`````",
	"texto", "## H", "", ":::note", ":::", "- item", "```mermaid", ":::code-group", "```js [a]",
}

// mainCodeLines es la regla de las versiones anteriores, escrita aparte:
// abre toda línea que empieza con ```, cierra la primera ``` exacta, y sin
// cierre llega al final. Devuelve las líneas de código y si alguna cerca
// quedó sin cerrar.
func mainCodeLines(lines []string) ([]bool, bool) {
	marks := make([]bool, len(lines))
	for i := 0; i < len(lines); i++ {
		if !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
			continue
		}
		end := -1
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "```" {
				end = j
				break
			}
		}
		if end < 0 {
			for j := i; j < len(lines); j++ {
				marks[j] = true
			}
			return marks, true
		}
		for j := i; j <= end; j++ {
			marks[j] = true
		}
		i = end
	}
	return marks, false
}

// commonMarkUnclosed dice si leyendo lines solo con CommonMark (backticks)
// alguna cerca queda sin cerrar. Escrita aparte de internal/fence.
func commonMarkUnclosed(lines []string) bool {
	run := func(s string) int { return len(s) - len(strings.TrimLeft(s, "`")) }
	for i := 0; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		n := run(t)
		if n < 3 || strings.Contains(t[n:], "`") {
			continue
		}
		end := -1
		for j := i + 1; j < len(lines); j++ {
			u := strings.TrimSpace(lines[j])
			if m := run(u); m == len(u) && m >= n {
				end = j
				break
			}
		}
		if end < 0 {
			return true
		}
		i = end
	}
	return false
}

// FuzzFenceReadingIsReported arma documentos con fenceTokens y exige:
//
//   - si las líneas de código no son las que daba la regla de las versiones
//     anteriores, hay un aviso CODE002, CODE003 o CODE004: ningún documento
//     cambia de significado en silencio;
//   - si la regla anterior leía el documento sin cercas abiertas y CommonMark
//     dejaría una abierta, hay un CODE003; y si ninguna cerca bien formada se
//     lee distinto que antes (sin CODE002 ni CODE004), las líneas de código
//     son las de la regla anterior: el resultado es el de antes, más el aviso.
//
// Solo cuenta el código que guarda CodeParser o un code-group (lo que marca
// CodeLiteralLines); las cercas de diagrama no son código.
func FuzzFenceReadingIsReported(f *testing.F) {
	f.Add([]byte{3, 7, 0, 8})
	f.Add([]byte{6, 7, 1, 7, 0})
	f.Add([]byte{5, 7, 0, 8, 2, 7, 1})
	f.Add([]byte{3, 7, 0, 8, 3, 7, 3})
	f.Add([]byte{14, 15, 7, 0, 11})
	f.Fuzz(func(t *testing.T, picks []byte) {
		if len(picks) > 40 {
			picks = picks[:40]
		}
		body := make([]string, len(picks))
		for i, p := range picks {
			body[i] = fenceTokens[int(p)%len(fenceTokens)]
		}
		src := "# A\n\n" + strings.Join(body, "\n")
		lines := strings.Split(src, "\n")
		p := New(util.NewNoop())
		p.SetNormalization(false)
		_, diags := p.ParseDocument(src, "")
		reported, fallback, changed := false, false, false
		for _, d := range diags {
			switch d.RuleID {
			case "CODE002", "CODE004":
				reported, changed = true, true
			case "CODE003":
				reported, fallback = true, true
			}
		}
		got := CodeLiteralLines(src)
		want, mainUnclosed := mainCodeLines(lines)
		// Las cercas de diagrama no son código para CodeLiteralLines: la
		// regla anterior se compara solo en documentos sin ellas.
		if strings.Contains(src, "```mermaid") {
			return
		}
		same := true
		for i := range lines {
			// La cabecera de un code-group es código para CodeLiteralLines.
			if t := strings.TrimSpace(lines[i]); got[i] != want[i] && t != "" && !strings.HasPrefix(t, ":::code-group") {
				same = false
			}
		}
		if !same && !reported {
			t.Fatalf("code lines changed from the earlier rule without a CODE00x warning:\n%s\n got %v\nwant %v", src, got, want)
		}
		if !mainUnclosed && commonMarkUnclosed(lines) {
			if !fallback {
				t.Fatalf("CommonMark leaves a fence open but there is no CODE003:\n%s\n%v", src, diags)
			}
			if !changed && !same {
				t.Fatalf("only the fallback applies, so the code lines must be the earlier rule's:\n%s\n got %v\nwant %v", src, got, want)
			}
		}
	})
}
