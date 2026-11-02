// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/include"
	"go.ziradocs.com/core/v2/renderer"
)

// rebaseIncludedAssets reescribe las fuentes locales relativas que vienen de
// un archivo incluido para que, resueltas desde sourceDir como todas las
// demás, apunten a lo que el autor escribió relativo a SU archivo. Sin esto,
// `![](logo.png)` dentro de chap/x.md se buscaba junto al archivo raíz: la
// expansión de @include es textual y los generadores solo conocen un
// directorio base.
//
// Cubre las imágenes de bloque y los videos/audios con su póster (campos del
// AST), y las imágenes inline `![alt](src)` dentro de la prosa que los
// generadores leen como Markdown inline: párrafos, ítems de lista y de
// checklist, celdas, citas, bloques especiales, columnas de grid, quizzes y
// encuestas. La prosa de un bloque de código no se toca (no está en esos
// campos), ni una imagen dentro de un code span.
//
// El origen de cada elemento sale de cruzar la línea de su posición con los
// orígenes de include.ExpandWithOrigins. No toca fuentes del archivo raíz,
// de un incluido que está en el mismo sourceDir (el resultado sería el
// mismo), remotas, absolutas ni con {{variables}} (se sustituyen después, al
// renderizar, y el valor final no se conoce acá). Solo reescribe si la
// fuente aparece tal cual en una línea de ese mismo archivo a partir de la
// posición del elemento: si una posición no cuadrara con el texto, la fuente
// se queda como está (el comportamiento anterior) en vez de reescribirse
// contra el directorio de otro archivo.
//
// En HTML en modo browser el src reescrito queda relativo a sourceDir (el
// directorio del archivo raíz, o --base-dir), igual que una imagen escrita
// en el archivo raíz.
//
// No amplía nada: la ruta reescrita pasa por el mismo confinamiento a la
// raíz de imágenes que cualquier otra, así que un ".." que en el incluido
// salía de la raíz sigue saliendo.
func rebaseIncludedAssets(doc *ast.AST, lines []string, origins include.LineOrigins, sourceDir string) {
	if doc == nil || sourceDir == "" || len(origins) == 0 {
		return
	}
	r := assetRebaser{lines: lines, origins: origins, sourceDir: sourceDir}
	_ = ast.Walk(doc, func(n ast.Node) error {
		el, ok := n.(ast.Element)
		if !ok {
			return nil
		}
		line := el.GetPosition().Line
		file := origins.File(line)
		if file == "" || filepath.Dir(file) == sourceDir {
			return nil
		}
		dir := filepath.Dir(file)
		src := func(v string) string { return r.source(v, line, file, dir) }
		txt := func(v string) string { return r.inline(v, line, file, dir) }
		switch e := n.(type) {
		case *ast.ImageElement:
			e.Source = src(e.Source)
		case *ast.MediaElement:
			e.Source = src(e.Source)
			e.Poster = src(e.Poster)
		case *ast.TextElement:
			e.Content = txt(e.Content)
		case *ast.PointsElement:
			rebasePointItems(e.Items, txt)
		case *ast.ChecklistElement:
			rebaseChecklistItems(e.Items, txt)
		case *ast.QuoteElement:
			e.Content = txt(e.Content)
		case *ast.SpecialBlockElement:
			e.Content = txt(e.Content)
		case *ast.GridElement:
			e.Content = txt(e.Content)
			for i := range e.Columns {
				e.Columns[i].Content = txt(e.Columns[i].Content)
			}
		case *ast.TableElement:
			for i := range e.Headers {
				e.Headers[i] = txt(e.Headers[i])
			}
			for _, row := range e.Rows {
				for j := range row {
					row[j] = txt(row[j])
				}
			}
			for _, row := range e.Cells {
				for j := range row {
					row[j].Content = txt(row[j].Content)
				}
			}
		case *ast.QuizElement:
			e.Question = txt(e.Question)
			e.Explanation = txt(e.Explanation)
			for i := range e.Options {
				e.Options[i] = txt(e.Options[i])
			}
		case *ast.PollElement:
			e.Question = txt(e.Question)
			for i := range e.Options {
				e.Options[i] = txt(e.Options[i])
			}
		}
		return nil
	})
}

func rebasePointItems(items []ast.PointItem, txt func(string) string) {
	for i := range items {
		items[i].Content = txt(items[i].Content)
		rebasePointItems(items[i].SubPoints, txt)
	}
}

func rebaseChecklistItems(items []ast.ChecklistItem, txt func(string) string) {
	for i := range items {
		items[i].Content = txt(items[i].Content)
		rebaseChecklistItems(items[i].SubItems, txt)
	}
}

// maxOriginScan acota cuántas líneas del mismo archivo se miran, a partir de
// la posición de un elemento, buscando el texto de una fuente.
const maxOriginScan = 400

type assetRebaser struct {
	lines     []string
	origins   include.LineOrigins
	sourceDir string
}

// seenInFile dice si needle aparece en una línea de file a partir de line,
// sin salir de las líneas contiguas que vienen de ese archivo.
func (r assetRebaser) seenInFile(needle string, line int, file string) bool {
	for l := line; l >= 1 && l <= len(r.lines) && l < line+maxOriginScan; l++ {
		if r.origins.File(l) != file {
			return false
		}
		if strings.Contains(r.lines[l-1], needle) {
			return true
		}
	}
	return false
}

func (r assetRebaser) rebasePath(src, dir string) (string, bool) {
	if !isRelativeLocalSource(src) {
		return src, false
	}
	rel, err := filepath.Rel(r.sourceDir, filepath.Join(dir, filepath.FromSlash(src)))
	if err != nil {
		return src, false
	}
	return filepath.ToSlash(rel), true
}

// source reescribe la fuente de un campo del AST (imagen de bloque, video,
// póster).
func (r assetRebaser) source(src string, line int, file, dir string) string {
	if src == "" || !r.seenInFile(src, line, file) {
		return src
	}
	out, _ := r.rebasePath(src, dir)
	return out
}

// inline reescribe el destino de cada ![alt](dest) de un texto Markdown
// inline, fuera de los code spans. dest es el destino crudo que reconoce el
// renderer (renderer.ScanLinkDestination): una ruta, opcionalmente entre
// <...> y seguida de un título; solo se cambia la ruta. Un destino con
// escapes (\) o referencias de caracteres (&) se deja como está, porque su
// ruta efectiva no es el texto literal.
func (r assetRebaser) inline(text string, line int, file, dir string) string {
	if !strings.Contains(text, "![") {
		return text
	}
	var b strings.Builder
	i := 0
	for i < len(text) {
		switch {
		case text[i] == '`':
			end := codeSpanEnd(text, i)
			b.WriteString(text[i:end])
			i = end
		case strings.HasPrefix(text[i:], "!["):
			m := inlineImageOpen.FindStringIndex(text[i:])
			if m == nil || m[0] != 0 {
				b.WriteString("![")
				i += 2
				continue
			}
			openEnd := i + m[1]
			dest, n, ok := renderer.ScanLinkDestination(text[openEnd:])
			if !ok {
				b.WriteString(text[i:openEnd])
				i = openEnd
				continue
			}
			b.WriteString(text[i:openEnd])
			b.WriteString(r.rebaseDestination(dest, line, file, dir))
			b.WriteString(text[openEnd+len(dest) : openEnd+n])
			i = openEnd + n
		default:
			b.WriteByte(text[i])
			i++
		}
	}
	return b.String()
}

var inlineImageOpen = regexp.MustCompile(`^!\[[^\]]*\]\(`)

func (r assetRebaser) rebaseDestination(dest string, line int, file, dir string) string {
	if strings.ContainsAny(dest, `\&`) || !r.seenInFile("("+dest+")", line, file) {
		return dest
	}
	lead := len(dest) - len(strings.TrimLeft(dest, " \t"))
	rest := dest[lead:]
	var path, tail, open, close string
	if strings.HasPrefix(rest, "<") {
		end := strings.Index(rest, ">")
		if end < 0 {
			return dest
		}
		open, path, close, tail = "<", rest[1:end], ">", rest[end+1:]
	} else if k := strings.IndexAny(rest, " \t"); k >= 0 {
		path, tail = rest[:k], rest[k:]
	} else {
		path = rest
	}
	newPath, ok := r.rebasePath(path, dir)
	if !ok || (open == "" && strings.ContainsAny(newPath, " \t")) {
		return dest
	}
	return dest[:lead] + open + newPath + close + tail
}

// codeSpanEnd devuelve dónde termina el code span que abre en text[i] (una
// corrida de backticks cerrada por otra del mismo largo), o el final de la
// corrida si no se cierra: entonces los backticks son literales y lo que
// sigue es texto normal.
func codeSpanEnd(text string, i int) int {
	j := i
	for j < len(text) && text[j] == '`' {
		j++
	}
	run := text[i:j]
	for k := j; k < len(text); {
		idx := strings.Index(text[k:], run)
		if idx < 0 {
			return j
		}
		s := k + idx
		e := s + len(run)
		if e < len(text) && text[e] == '`' {
			for e < len(text) && text[e] == '`' {
				e++
			}
			k = e
			continue
		}
		if s > 0 && text[s-1] == '`' {
			k = e
			continue
		}
		return e
	}
	return j
}

func isRelativeLocalSource(src string) bool {
	if src == "" || strings.Contains(src, "{{") || strings.HasPrefix(src, "/") || strings.HasPrefix(src, "#") || filepath.IsAbs(src) {
		return false
	}
	u, err := url.Parse(src)
	return err == nil && u.Scheme == ""
}
