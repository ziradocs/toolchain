// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// MaxTitleRunes es el largo máximo, en caracteres (runas) y ya normalizado,
// de --title y del `title` de la tool export. Un título es una línea: uno de
// miles de caracteres no es un título, y el PDF lo paga (con 60 000 el build
// pasa de 1.6 s a 16 s).
const MaxTitleRunes = 500

// errBlankTitle es el error de un --title (o el `title` de la tool export)
// que se pasó vacío, con solo espacios o con solo caracteres invisibles: un
// título en blanco es un error de uso, no un "sin título", igual que un
// --format o un --render-mode que no existen.
var errBlankTitle = errors.New("the title cannot be empty, only whitespace or only invisible characters")

// errTitleTooLong es el error de un título de más de MaxTitleRunes.
var errTitleTooLong = fmt.Errorf("the title cannot be longer than %d characters", MaxTitleRunes)

// isBidiControl dice si r es un control bidireccional de embedding,
// override o isolate (U+202A–U+202E, U+2066–U+2069): dentro de un
// encabezado o pie donde entra `{{title}}` reordenaría el texto que lo
// rodea, así que se quita.
func isBidiControl(r rune) bool {
	return (r >= '\u202A' && r <= '\u202E') || (r >= '\u2066' && r <= '\u2069')
}

// isLineBreak dice si r se cambia por un espacio: los caracteres de control
// (saltos de línea, tabuladores, C0 y C1) y los separadores de línea y de
// párrafo Unicode.
func isLineBreak(r rune) bool {
	return unicode.IsControl(r) || r == '\u2028' || r == '\u2029'
}

// NormalizeTitle prepara el valor de --title (o del `title` de la tool
// export) para usarlo como título del documento:
//
//   - una secuencia que no es UTF-8 válido (un surrogate suelto que llegó
//     como bytes, un argumento en otra codificación) se cambia por U+FFFD;
//   - cada tramo de caracteres de control (saltos de línea, tabuladores,
//     C0 y C1) y de separadores de línea o párrafo Unicode se cambia por un
//     espacio: el título es una sola línea en todas sus superficies (el
//     <title> del HTML, el /Title del PDF, el dc:title del DOCX), y un
//     carácter de control en un w:t dejaría el DOCX como XML inválido;
//   - se quitan los controles bidireccionales (isBidiControl);
//   - se recortan los espacios de los extremos.
//
// Un valor que queda vacío o hecho solo de espacios y caracteres de formato
// invisibles (categoría Cf: BOM, espacio de ancho cero…) es errBlankTitle;
// uno de más de MaxTitleRunes, errTitleTooLong. El valor no pasa por YAML
// ni por el parser: el escapado de cada salida lo hace su generador, igual
// que con el `title:` del frontmatter.
func NormalizeTitle(raw string) (string, error) {
	valid := strings.ToValidUTF8(raw, "\uFFFD")
	var b strings.Builder
	b.Grow(len(valid))
	inBreak := false
	for _, r := range valid {
		if isBidiControl(r) {
			continue
		}
		if isLineBreak(r) {
			if !inBreak {
				b.WriteByte(' ')
				inBreak = true
			}
			continue
		}
		inBreak = false
		b.WriteRune(r)
	}
	title := strings.TrimSpace(b.String())
	visible := strings.IndexFunc(title, func(r rune) bool {
		return !unicode.IsSpace(r) && !unicode.Is(unicode.Cf, r)
	}) >= 0
	if !visible {
		return "", errBlankTitle
	}
	if utf8.RuneCountInString(title) > MaxTitleRunes {
		return "", errTitleTooLong
	}
	return title, nil
}

// ValidateTitle revisa --title contra el formato de salida. Markdown no lo
// acepta: es la salida de ida y vuelta del documento, y su título ES el
// frontmatter que escribe (`title:` y el `# título` que agrega, que además
// baja un nivel cada sección). Ponerle un título que la fuente no tiene
// cambiaría el cuerpo, no solo el título.
func ValidateTitle(title, format string) error {
	if title != "" && (format == "markdown" || format == "md") {
		return fmt.Errorf("--title does not apply to --format %s: the Markdown output writes the document's own front matter, and a title the source does not declare would add a front matter block and demote every section", format)
	}
	return nil
}

// withTitle es el documento que ven los generadores con el título title,
// diga lo que diga la fuente (--title). Es el mismo efecto que un `title:`
// en el frontmatter sobre las superficies del título: el <title> y el
// encabezado de page-view del HTML, el /Title y el encabezado corrido del
// PDF, el dc:title y el párrafo de título del DOCX, y `{{title}}`
// (ast.FrontMatterNode.BuildVariables) donde ese formato ya lo sustituye.
//
// No toca el AST de la etapa Prepare: se arma una copia superficial del
// AST y del frontmatter, así que lo que devuelven get_ast y un --filter, el
// lint y sus líneas, y la normalización de la fuente (que ya corrió, sobre
// la fuente tal cual) quedan como sin la bandera. Se aplica DESPUÉS de
// ResolveTOCAndNumbering, así que un documento sin frontmatter no prende
// por esto el TOC, la numeración ni el visor interactivo: el frontmatter
// que se arma acá solo lleva el título. Un frontmatter que la fuente sí
// tiene conserva todo lo demás.
func withTitle(doc *ast.AST, title string) *ast.AST {
	if doc == nil || title == "" {
		return doc
	}
	copied := *doc
	var fm ast.FrontMatterNode
	if doc.FrontMatter != nil {
		fm = *doc.FrontMatter
	} else {
		fm = *ast.NewFrontMatterNode(diagnostics.NewPosition(1, 1))
	}
	fm.Title = title
	copied.FrontMatter = &fm
	return &copied
}
