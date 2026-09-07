// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package content

import (
	"strings"
	"testing"
)

func applyInlineHTML(t *testing.T, content string) string {
	t.Helper()
	out, err := NewInlineHTMLTagsRule().Apply(content)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	return out
}

func TestInlineHTMLTagsRule_RewritesEachTag(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{"underline", "Usa <u>esto</u> ahora.", "Usa [esto]{.underline} ahora."},
		{"sub", "H<sub>2</sub>O", "H[2]{.sub}O"},
		{"sup", "x<sup>2</sup>", "x[2]{.sup}"},
		{"kbd", "Pulsa <kbd>Ctrl</kbd>", "Pulsa [Ctrl]{.kbd}"},
		{"mark", "Esto es <mark>clave</mark>.", "Esto es ==clave==."},
		{"small", "<small>nota al pie</small>", "[nota al pie]{.small}"},
		{"code", "Corre <code>go build</code>", "Corre `go build`"},
		{"mayúsculas", "Usa <U>esto</U>", "Usa [esto]{.underline}"},
		{"con atributos", `<u class="x" id="y">esto</u>`, "[esto]{.underline}"},
		{"varios en una línea", "<u>a</u> y <kbd>b</kbd>", "[a]{.underline} y [b]{.kbd}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := applyInlineHTML(t, tc.in); got != tc.want {
				t.Errorf("Apply(%q) = %q, se esperaba %q", tc.in, got, tc.want)
			}
		})
	}
}

// Lo que NO debe tocar. Cada caso es contenido que hoy funciona y que una
// reescritura ingenua rompería.
func TestInlineHTMLTagsRule_LeavesTheseAlone(t *testing.T) {
	for _, tc := range []struct {
		name, in string
	}{
		{"tag sin cerrar", "Un <u>texto que sigue"},
		{"cierre sin apertura", "texto</u> suelto"},
		{"par en líneas distintas", "Empieza <u>acá\ny cierra</u> allá"},
		{"interior con corchetes", "<u>[a](b)</u>"},
		{"interior con otra tag", "<u>texto <b>negrita</b></u>"},
		{"tag no incluida", "<b>negrita</b> y <i>cursiva</i>"},
		{"prefijo parecido", "<underline>x</underline>"},
		{"ya es token", "[esto]{.underline}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := applyInlineHTML(t, tc.in); got != tc.in {
				t.Errorf("Apply(%q) tocó la línea: %q", tc.in, got)
			}
		})
	}
}

// Dentro de un fence el HTML literal ES el contenido — es el caso de un slide
// que enseña HTML.
func TestInlineHTMLTagsRule_SkipsCodeFences(t *testing.T) {
	in := "Antes <u>sí</u>.\n\n```html\n<u>no tocar</u>\n<kbd>tampoco</kbd>\n```\n\nDespués <u>sí</u>."
	got := applyInlineHTML(t, in)

	if strings.Count(got, "<u>no tocar</u>") != 1 {
		t.Errorf("se reescribió dentro del fence:\n%s", got)
	}
	if !strings.Contains(got, "<kbd>tampoco</kbd>") {
		t.Errorf("se reescribió <kbd> dentro del fence:\n%s", got)
	}
	if strings.Count(got, "[sí]{.underline}") != 2 {
		t.Errorf("no se reescribió fuera del fence:\n%s", got)
	}
}

func TestInlineHTMLTagsRule_SkipsEmbeddedBlocks(t *testing.T) {
	in := "Antes <u>sí</u>.\n\n<<chart: bar>>\ntitle: \"<u>no tocar</u>\"\n<<end>>\n\nDespués <u>sí</u>."
	got := applyInlineHTML(t, in)

	if !strings.Contains(got, `title: "<u>no tocar</u>"`) {
		t.Errorf("se reescribió dentro del bloque embebido:\n%s", got)
	}
	if strings.Count(got, "[sí]{.underline}") != 2 {
		t.Errorf("no se reescribió fuera del bloque:\n%s", got)
	}
}

// El frontmatter no es prosa: sus valores llegan tipados al AST.
func TestInlineHTMLTagsRule_SkipsFrontmatter(t *testing.T) {
	in := "---\ntitle: \"<u>no tocar</u>\"\n---\n\nCuerpo <u>sí</u>."
	got := applyInlineHTML(t, in)

	if !strings.Contains(got, `title: "<u>no tocar</u>"`) {
		t.Errorf("se tocó el frontmatter:\n%s", got)
	}
	if !strings.Contains(got, "[sí]{.underline}") {
		t.Errorf("no se reescribió el cuerpo:\n%s", got)
	}
}

// Un `<code>` que envuelve otra tag tiene que conservarla literal: es el motivo
// de que code vaya primero en la lista de reescrituras.
func TestInlineHTMLTagsRule_CodeContentIsNotReinterpreted(t *testing.T) {
	// El interior trae `<`/`>`, así que ninguna regla lo toca — ni code ni u.
	in := "Escribe <code><u>x</u></code> así"
	if got := applyInlineHTML(t, in); got != in {
		t.Errorf("Apply(%q) = %q; el interior con tags no debe reescribirse", in, got)
	}
}

// Sin cambios, Apply devuelve el contenido original tal cual (el normalizador
// compara antes/después para decidir si la regla "aplicó").
func TestInlineHTMLTagsRule_NoChangeReturnsOriginal(t *testing.T) {
	in := "Prosa normal sin tags.\n\nOtra línea."
	if got := applyInlineHTML(t, in); got != in {
		t.Errorf("Apply cambió contenido sin tags: %q", got)
	}
}

// Un tramo entre backticks es código: su interior es literal aunque traiga una
// tag que la regla reescribiría en prosa.
func TestInlineHTMLTagsRule_BacktickCodeIsProtected(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{
			"backticks protegen",
			"Escribe `<u>x</u>` para subrayar",
			"Escribe `<u>x</u>` para subrayar",
		},
		{
			"prosa fuera, código adentro",
			"Usa <u>esto</u> pero no `<u>esto</u>`",
			"Usa [esto]{.underline} pero no `<u>esto</u>`",
		},
		{
			"code tag con tag adentro queda literal",
			"Escribe <code><u>x</u></code> así",
			"Escribe <code><u>x</u></code> así",
		},
		{
			"code tag limpia pasa a backticks",
			"Corre <code>go build</code>",
			"Corre `go build`",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := applyInlineHTML(t, tc.in); got != tc.want {
				t.Errorf("Apply(%q) = %q, se esperaba %q", tc.in, got, tc.want)
			}
		})
	}
}

// El centinela de protección nunca puede quedar en la salida.
func TestInlineHTMLTagsRule_NoSentinelLeaks(t *testing.T) {
	for _, in := range []string{
		"`a` y `b` y <u>c</u>",
		"<code>x</code> <code><u>y</u></code> <kbd>z</kbd>",
		"backtick suelto ` sin cerrar y <u>x</u>",
	} {
		got := applyInlineHTML(t, in)
		if strings.Contains(got, "zdcode") || strings.Contains(got, "\x00") {
			t.Errorf("se filtró un centinela en %q → %q", in, got)
		}
	}
}

// Un backtick dentro de un <code> rompía la reescritura: "<code>a`b</code>"
// salía como "`a`b`", que se lee como código "a" seguido de "b`" suelto.
func TestInlineHTMLTagsRule_BacktickInsideCodeIsNotRewritten(t *testing.T) {
	for _, in := range []string{
		"Escribe <code>a`b</code> así",
		"Corre <code>echo `date`</code> y listo",
	} {
		if got := applyInlineHTML(t, in); got != in {
			t.Errorf("Apply(%q) = %q; un <code> con backtick debe quedar literal", in, got)
		}
	}
}

// Un bloque embebido terminado por una frontera de documento (no por
// `<<end>>`) apagaba la regla para TODO el resto del archivo. Y `---` es el
// separador de slides, o sea la forma más común de que eso pase.
func TestInlineHTMLTagsRule_BoundaryReopensAfterEmbeddedBlock(t *testing.T) {
	for _, boundary := range []string{"---", "# Otro slide", "## Otra sección"} {
		in := "<<chart: bar>>\ndata: [1]\n" + boundary + "\n\nTexto con <u>subrayado</u>."
		got := applyInlineHTML(t, in)
		if !strings.Contains(got, "[subrayado]{.underline}") {
			t.Errorf("tras %q la regla quedó apagada:\n%s", boundary, got)
		}
		if !strings.Contains(got, "data: [1]") {
			t.Errorf("tras %q se tocó el cuerpo del bloque:\n%s", boundary, got)
		}
	}
}
