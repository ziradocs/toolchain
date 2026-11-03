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
// spans. El renderer HTML y el DOCX aceptan `![](a b.png)` (con el core
// actual, con un aviso LINK001), pero CommonMark no: en la salida Markdown,
// que leen otras herramientas, esa imagen dejaría de ser imagen. Aparece
// sobre todo cuando una imagen de un archivo incluido se reescribe relativa
// al archivo raíz y la carpeta del incluido tiene espacios.
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
	d, ok := SplitMarkdownDestination(dest)
	// Spaced: core lee la ruta con la regla anterior a CommonMark (una ruta
	// suelta con espacios, o con algo que parece un título y no lo es, como
	// `a (1).png`). Entre <...> la lee igual cualquier herramienta.
	if !ok || !d.Spaced || strings.ContainsAny(d.Path, "<>") {
		return dest
	}
	return d.Lead + "<" + d.Path + ">" + d.Tail
}

// MarkdownDestinationParts es el destino crudo de un enlace o imagen inline
// (lo que devuelve renderer.ScanLinkDestination) partido como lo lee core.
// Lead + Open + Path + Close + Tail es el destino original, byte a byte.
type MarkdownDestinationParts struct {
	// Lead son los espacios antes del destino.
	Lead string
	// Open y Close son "<" y ">" en un destino entre <...>, o vacíos.
	Open, Close string
	// Path es la ruta tal como está escrita (sin decodificar).
	Path string
	// Tail es lo que sigue a la ruta: espacios y el título, tal cual.
	Tail string
	// Spaced dice que core leyó la ruta con la regla de compatibilidad
	// (ver renderer.LinkDestination.Spaced).
	Spaced bool
}

// SplitMarkdownDestination parte dest con renderer.ParseLinkDestination, el
// escáner del renderer HTML, en vez de adivinar dónde empieza el título. Un
// paréntesis después de un espacio es título solo si es un título bien
// formado al final del destino: en `fotos/a (1).png` no lo es (después del
// ")" sigue texto) y la ruta es todo; en `c d.png 'e` tampoco (no cierra).
// En `mi dir/x.png "T"` sí: core lee la ruta entera con la regla de
// compatibilidad, y acá se separa la ruta del título (splitSpacedTitle). ok
// es false si dest no es un destino que core lea entero.
func SplitMarkdownDestination(dest string) (MarkdownDestinationParts, bool) {
	ld, ok := renderer.ParseLinkDestination(dest + ")")
	if !ok || ld.Consumed != len(dest)+1 {
		return MarkdownDestinationParts{}, false
	}
	lead := len(dest) - len(strings.TrimLeft(dest, " \t"))
	p := MarkdownDestinationParts{Lead: dest[:lead], Path: ld.Dest, Spaced: ld.Spaced}
	start := lead
	if strings.HasPrefix(dest[lead:], "<") && !ld.Spaced {
		p.Open, p.Close = "<", ">"
		start++
	}
	end := start + len(ld.Dest)
	if end > len(dest) || dest[start:end] != ld.Dest || !strings.HasPrefix(dest[end:], p.Close) {
		return MarkdownDestinationParts{}, false
	}
	p.Tail = dest[end+len(p.Close):]
	// Una ruta suelta con espacios seguida de un título bien formado
	// (`mi dir/x.png "T"`): core la lee entera con la regla de
	// compatibilidad, pero el autor escribió una ruta y un título, y así la
	// leían el DOCX y la salida Markdown. Se separan (ver splitSpacedTitle).
	if p.Spaced {
		if path, rest, ok := splitSpacedTitle(p.Path); ok {
			p.Path, p.Tail = path, rest+p.Tail
		}
	}
	return p, true
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
// que el escáner de core vuelve a leer igual: suelto si se lee entero y no
// lleva espacios, si no entre <...> (espacios, un ")" desbalanceado). Un path con < o > no tiene
// forma entre <...> y se escribe suelto.
func markdownDestination(path, title string) string {
	dest := path
	if raw, n, ok := renderer.ScanLinkDestination(path + ")"); (!ok || n != len(path)+1 || raw != path || strings.ContainsAny(path, " \t")) && !strings.ContainsAny(path, "<>\r\n") {
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

// splitSpacedTitle parte una ruta suelta con espacios que core leyó con la
// regla de compatibilidad (renderer.LinkDestination.Spaced) cuando termina
// en un título bien formado de CommonMark: "…", '…' o (…), con sus escapes,
// separado de la ruta por espacios. `mi dir/x.png "T"` es la ruta
// `mi dir/x.png` con el título "T", como lo escribió el autor y como lo
// leían el DOCX y la salida Markdown hasta v2.49.1. `a (1).png` no lo parte:
// después del ")" sigue texto, así que "(1).png" no es un título. `c d.png 'e`
// tampoco: el título no cierra.
//
// Devuelve la ruta y el resto (los espacios y el título, tal cual).
func splitSpacedTitle(p string) (path, rest string, ok bool) {
	n := len(p)
	if n < 2 {
		return "", "", false
	}
	escaped := make([]bool, n)
	for i := 0; i < n; i++ {
		if p[i] == '\\' && i+1 < n && isASCIIPunct(p[i+1]) {
			escaped[i+1] = true
			i++
		}
	}
	last := p[n-1]
	if escaped[n-1] {
		return "", "", false
	}
	var open byte
	switch last {
	case '"', '\'':
		open = last
	case ')':
		open = '('
	default:
		return "", "", false
	}
	k := -1
	for i := n - 2; i >= 0; i-- {
		if escaped[i] {
			continue
		}
		if p[i] == '\n' || p[i] == '\r' {
			return "", "", false
		}
		if p[i] == open {
			k = i
			break
		}
		// Un título entre paréntesis no lleva "(" ni ")" sin escapar.
		if open == '(' && p[i] == ')' {
			return "", "", false
		}
	}
	if k <= 0 || (p[k-1] != ' ' && p[k-1] != '\t') {
		return "", "", false
	}
	path = strings.TrimRight(p[:k], " \t")
	if path == "" {
		return "", "", false
	}
	return path, p[len(path):], true
}
