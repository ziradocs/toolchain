// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
)

var markdownImageOpen = regexp.MustCompile(`^!\[[^\]]*\]\(`)

// MarkdownImageDestinations envuelve en <...> la ruta de cada ![alt](dest)
// de md que lleva espacios sin estar ya entre <...>, fuera de los code
// spans. Con un core anterior a los destinos CommonMark, el renderer HTML y
// el DOCX aceptan `![](a b.png)`, pero CommonMark no: en la salida Markdown,
// que leen otras herramientas, esa imagen dejaría de ser imagen. Con el core
// actual ese destino ya no es imagen en ninguna salida (ScanLinkDestination
// lo rechaza) y esta función no lo toca; las rutas con espacios que escribe
// doclang ya van entre <...>.
func MarkdownImageDestinations(md string) string {
	if !strings.Contains(md, "![") {
		return md
	}
	var b strings.Builder
	i := 0
	for i < len(md) {
		switch {
		case md[i] == '`':
			end := MarkdownCodeSpanEnd(md, i)
			b.WriteString(md[i:end])
			i = end
		case strings.HasPrefix(md[i:], "!["):
			m := markdownImageOpen.FindStringIndex(md[i:])
			if m == nil {
				b.WriteString("![")
				i += 2
				continue
			}
			openEnd := i + m[1]
			dest, n, ok := renderer.ScanLinkDestination(md[openEnd:])
			if !ok {
				b.WriteString(md[i:openEnd])
				i = openEnd
				continue
			}
			b.WriteString(md[i:openEnd])
			b.WriteString(angleDestination(dest))
			b.WriteString(md[openEnd+len(dest) : openEnd+n])
			i = openEnd + n
		default:
			b.WriteByte(md[i])
			i++
		}
	}
	return b.String()
}

func angleDestination(dest string) string {
	lead := len(dest) - len(strings.TrimLeft(dest, " \t"))
	rest := dest[lead:]
	if strings.HasPrefix(rest, "<") {
		return dest
	}
	path, tail := rest, ""
	if k := MarkdownTitleStart(rest); k >= 0 {
		path, tail = rest[:k], rest[k:]
	}
	if !strings.ContainsAny(path, " \t") || strings.ContainsAny(path, "<>") {
		return dest
	}
	return dest[:lead] + "<" + path + ">" + tail
}

// MarkdownTitleStart devuelve dónde empieza el título de un destino de
// enlace sin <...> (el espacio antes de una comilla o un paréntesis), o -1
// si no tiene. Un espacio que no va seguido de un título es parte de la
// ruta, igual que lo lee el renderer HTML.
func MarkdownTitleStart(dest string) int {
	for k := 0; k < len(dest); k++ {
		if dest[k] != ' ' && dest[k] != '\t' {
			continue
		}
		rest := strings.TrimLeft(dest[k:], " \t")
		if rest != "" && strings.ContainsRune(`"'(`, rune(rest[0])) {
			return k
		}
	}
	return -1
}

// MarkdownCodeSpanEnd devuelve dónde termina el code span que abre en
// text[i] (una corrida de backticks cerrada por otra del mismo largo), o el
// final de la corrida si no se cierra: entonces los backticks son literales
// y lo que sigue es texto normal.
func MarkdownCodeSpanEnd(text string, i int) int {
	j := i
	for j < len(text) && text[j] == '`' {
		j++
	}
	n := j - i
	for k := j; k < len(text); {
		if text[k] != '`' {
			k++
			continue
		}
		e := k
		for e < len(text) && text[e] == '`' {
			e++
		}
		if e-k == n {
			return e
		}
		k = e
	}
	return j
}

// markdownBlockImageDestination escribe el destino de una imagen de bloque
// para la salida Markdown: la fuente entre <...> cuando lleva espacios (en
// CommonMark una ruta suelta con espacios no es imagen) y, si el AST trae
// título (image-title-v1), el título entre comillas. El título se lee con
// una aserción de interfaz para seguir compilando contra un core anterior;
// ese core deja el destino crudo en Source (título y <...> incluidos), que
// se escribe tal cual, como antes.
func markdownBlockImageDestination(elem *ast.ImageElement) string {
	t, ok := any(elem).(interface{ ImageTitle() string })
	if !ok {
		return elem.Source
	}
	return markdownDestination(elem.Source, t.ImageTitle())
}

// markdownDestination escribe path (y title, si hay) como destino Markdown
// que el escáner de core vuelve a leer igual: suelto si se lee entero, si no
// entre <...> (espacios, un ")" desbalanceado). Un path con < o > no tiene
// forma entre <...> y se escribe suelto.
func markdownDestination(path, title string) string {
	dest := path
	if raw, n, ok := renderer.ScanLinkDestination(path + ")"); (!ok || n != len(path)+1 || raw != path) && !strings.ContainsAny(path, "<>\r\n") {
		dest = "<" + path + ">"
	}
	if title != "" {
		dest += ` "` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(title) + `"`
	}
	return dest
}

// headingImgTagPattern es el <img> que el pipeline inline deja en el HTML de
// un encabezado (`## Título ![a](a.png "t")`).
var headingImgTagPattern = regexp.MustCompile(`<img src="([^"]*)" alt="([^"]*)"(?: title="([^"]*)")?>`)

// headingImagesToMarkdown vuelve a escribir como ![alt](src "título") cada
// imagen del HTML de un encabezado, para que la salida Markdown no la pierda
// al quitar el resto del markup (docxStripHeadingMarkup). Los valores ya
// vienen escapados para HTML y se desescapan.
func headingImagesToMarkdown(html string) string {
	if !strings.Contains(html, "<img ") {
		return html
	}
	return headingImgTagPattern.ReplaceAllStringFunc(html, func(tag string) string {
		m := headingImgTagPattern.FindStringSubmatch(tag)
		dest := markdownDestination(renderer.UnescapeHTML(m[1]), renderer.UnescapeHTML(m[3]))
		// El resultado pasa después por docxStripHeadingMarkup, que
		// desescapa HTML: se escapan los "&" para que sobrevivan tal cual.
		return strings.ReplaceAll("!["+renderer.UnescapeHTML(m[2])+"]("+dest+")", "&", "&amp;")
	})
}
