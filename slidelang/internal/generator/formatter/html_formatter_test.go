// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"
)

// indentOf retorna la cantidad de espacios de indentación al inicio de line.
func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}

// TestIndentHTML_MultiLineTagsBalanced cubre issue #12a: un tag contenedor
// cuyo '>' de apertura llega varias líneas después (atributos multilínea) y
// se cierra con un </div> pegado al final de esa última línea de atributos
// no debe consumir un nivel de indentación neto — antes, la heurística
// basada en prefijo de línea era ciega a ese cierre "pegado" y el nivel
// crecía +1 por cada elemento así (mermaid, map), desbalanceando los </div>
// de los slides siguientes.
func TestIndentHTML_MultiLineTagsBalanced(t *testing.T) {
	f := NewHTMLFormatter()

	html := strings.Join([]string{
		`<div class="slidelang-slide" id="slide-0">`,
		`<div class="slidelang-element slidelang-mermaid"`,
		`     id="slidelang-element-mermaid-wrapper-0-1"`,
		`     data-slide="0">`,
		`<div class="slidelang-mermaid"`,
		`     id="slidelang-element-mermaid-0-1"`,
		`     data-diagram-type="flowchart"></div>`,
		`</div>`,
		`</div>`,
		`<div class="slidelang-slide" id="slide-1">`,
		`<script>`,
		`if (a < b) { console.log("ok"); }`,
		`</script>`,
		`</div>`,
	}, "\n")

	formatted := f.indentHTML(html)
	lines := strings.Split(formatted, "\n")

	if len(lines) != 14 {
		t.Fatalf("expected 14 lines, got %d:\n%s", len(lines), formatted)
	}

	slide0Indent := indentOf(lines[0])
	slide1Indent := indentOf(lines[9])
	if slide0Indent != slide1Indent {
		t.Errorf("expected both top-level slide divs at the same indent, got %d and %d:\n%s", slide0Indent, slide1Indent, formatted)
	}

	// El div interno de mermaid (abre en la línea 4, cierra pegado en la 6)
	// no debe dejar el nivel desbalanceado: la línea que cierra el wrapper
	// (línea 7, "</div>") debe volver exactamente al nivel del wrapper (línea 1).
	wrapperOpenIndent := indentOf(lines[1])
	wrapperCloseIndent := indentOf(lines[7])
	if wrapperOpenIndent != wrapperCloseIndent {
		t.Errorf("mermaid wrapper open/close indent mismatch: open=%d close=%d (drift):\n%s", wrapperOpenIndent, wrapperCloseIndent, formatted)
	}

	// La segunda línea del slide 0 (cierre final del slide) debe volver al
	// mismo nivel que la apertura del slide.
	slide0CloseIndent := indentOf(lines[8])
	if slide0CloseIndent != slide0Indent {
		t.Errorf("slide 0 did not return to its own indent level after closing: open=%d close=%d:\n%s", slide0Indent, slide0CloseIndent, formatted)
	}

	// El "<" inline de "a < b" dentro del <script> no debe contarse como tag.
	scriptLine := lines[11]
	if !strings.Contains(scriptLine, "a < b") {
		t.Fatalf("expected the script line's content to survive untouched, got: %q", scriptLine)
	}
}

// TestIndentHTML_SimpleNesting verifica el caso común (no regresión): un
// contenedor simple que abre y cierra en líneas separadas incrementa y
// decrementa el nivel exactamente una vez.
func TestIndentHTML_SimpleNesting(t *testing.T) {
	f := NewHTMLFormatter()

	html := strings.Join([]string{
		`<div class="outer">`,
		`<div class="inner">`,
		`<p>text</p>`,
		`</div>`,
		`</div>`,
	}, "\n")

	formatted := f.indentHTML(html)
	lines := strings.Split(formatted, "\n")

	if indentOf(lines[0]) != 0 {
		t.Errorf("expected outer div at indent 0, got %d", indentOf(lines[0]))
	}
	if indentOf(lines[1]) != f.indentSize {
		t.Errorf("expected inner div at indent %d, got %d", f.indentSize, indentOf(lines[1]))
	}
	if indentOf(lines[2]) != f.indentSize*2 {
		t.Errorf("expected <p> at indent %d, got %d", f.indentSize*2, indentOf(lines[2]))
	}
	if indentOf(lines[3]) != f.indentSize {
		t.Errorf("expected inner </div> at indent %d, got %d", f.indentSize, indentOf(lines[3]))
	}
	if indentOf(lines[4]) != 0 {
		t.Errorf("expected outer </div> at indent 0, got %d", indentOf(lines[4]))
	}
}

// TestIndentHTML_SameLineOpenClose verifica que un tag contenedor que abre y
// cierra en la MISMA línea (p.ej. "<li>x</li>") no incrementa el nivel.
func TestIndentHTML_SameLineOpenClose(t *testing.T) {
	f := NewHTMLFormatter()

	html := strings.Join([]string{
		`<ul>`,
		`<li>one</li>`,
		`<li>two</li>`,
		`</ul>`,
	}, "\n")

	formatted := f.indentHTML(html)
	lines := strings.Split(formatted, "\n")

	if indentOf(lines[1]) != f.indentSize || indentOf(lines[2]) != f.indentSize {
		t.Errorf("expected both same-line <li> elements at indent %d, got %d and %d:\n%s", f.indentSize, indentOf(lines[1]), indentOf(lines[2]), formatted)
	}
	if indentOf(lines[3]) != 0 {
		t.Errorf("expected </ul> back at indent 0, got %d:\n%s", indentOf(lines[3]), formatted)
	}
}
