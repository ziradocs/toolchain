// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/include"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/doclang/v2/internal/generator"
)

// rebaseIncludedAssets reescribe las fuentes locales relativas que vienen de
// un archivo incluido para que, resueltas desde sourceDir como todas las
// demás, apunten a lo que el autor escribió relativo a SU archivo. Sin esto,
// `![](logo.png)` dentro de chap/x.md se buscaba junto al archivo raíz: la
// expansión de @include es textual y los generadores solo conocen un
// directorio base.
//
// Cubre las imágenes de bloque y los videos/audios con su póster (campos del
// AST), las imágenes inline `![alt](src)` de la prosa que los generadores
// leen como Markdown inline (párrafos, encabezados, ítems de lista y de
// checklist, celdas, citas, bloques especiales, columnas de grid, quizzes y
// encuestas) y el <img> que el parser ya dejó armado en el HTML de un
// encabezado. La prosa de un bloque de código no se toca (no está en esos
// campos), ni una imagen dentro de un code span.
//
// El origen se decide por ocurrencia, no por elemento: cada fuente se busca,
// en orden, en las líneas del texto expandido que le tocan a su campo (desde
// la posición del elemento, ítem o fila hasta la siguiente posición conocida
// del documento), y se reescribe según el archivo del que viene LA LÍNEA
// donde aparece. Así un párrafo o una lista que cruza el borde de un
// @include (un incluido sin salto de línea final, una lista del archivo raíz
// que sigue con un @include) reescribe solo lo que escribió el incluido. Una
// fuente que no aparece en esas líneas se queda como está (el comportamiento
// anterior) en vez de reescribirse contra el directorio de otro archivo.
//
// No toca fuentes del archivo raíz, de un incluido que está en el mismo
// sourceDir, remotas, absolutas ni con {{variables}} (se sustituyen al
// renderizar y el valor final no se conoce acá). Las rutas se comparan con
// los symlinks resueltos, así que da lo mismo si sourceDir y la raíz de
// includes llegan por caminos distintos al mismo directorio.
//
// En HTML en modo browser y offline-assets el src reescrito queda relativo a
// sourceDir (el directorio del archivo raíz, o --base-dir), igual que una
// imagen escrita en el archivo raíz.
//
// No amplía nada: la ruta reescrita pasa por el mismo confinamiento a la
// raíz de imágenes que cualquier otra, así que un ".." que en el incluido
// salía de la raíz sigue saliendo.
func rebaseIncludedAssets(doc *ast.AST, lines []string, origins include.LineOrigins, sourceDir string) {
	if doc == nil || sourceDir == "" || len(origins) == 0 {
		return
	}
	r := &assetRebaser{
		lines:     lines,
		origins:   origins,
		realDirs:  map[string]string{},
		sourceDir: sourceDir,
	}
	r.realSource = r.realDir(sourceDir)
	r.collectStarts(doc)
	_ = ast.Walk(doc, func(n ast.Node) error {
		el, ok := n.(ast.Element)
		if !ok {
			return nil
		}
		line := el.GetPosition().Line
		txt := func(v string) string { return r.inline(v, line) }
		switch e := n.(type) {
		case *ast.ImageElement:
			e.Source = r.newField(line).source(e.Source)
		case *ast.MediaElement:
			f := r.newField(line)
			e.Source = f.source(e.Source)
			e.Poster = f.source(e.Poster)
		case *ast.TextElement:
			e.Content = r.headingHTML(txt(e.Content), line)
		case *ast.HeadingElement:
			e.Text = r.headingHTML(txt(e.Text), line)
		case *ast.PointsElement:
			r.pointItems(e.Items)
		case *ast.ChecklistElement:
			r.checklistItems(e.Items)
		case *ast.QuoteElement:
			e.Content = txt(e.Content)
		case *ast.SpecialBlockElement:
			e.Content = txt(e.Content)
		case *ast.GridElement:
			e.Content = txt(e.Content)
			for i := range e.Columns {
				e.Columns[i].Content = r.inline(e.Columns[i].Content, e.Columns[i].Position.Line)
			}
		case *ast.TableElement:
			r.table(e, line)
		case *ast.QuizElement:
			f := r.newField(line)
			e.Question = f.inline(e.Question)
			for i := range e.Options {
				e.Options[i] = f.inline(e.Options[i])
			}
			e.Explanation = f.inline(e.Explanation)
		case *ast.PollElement:
			f := r.newField(line)
			e.Question = f.inline(e.Question)
			for i := range e.Options {
				e.Options[i] = f.inline(e.Options[i])
			}
		}
		return nil
	})
}

type assetRebaser struct {
	lines      []string
	origins    include.LineOrigins
	sourceDir  string
	realSource string
	realDirs   map[string]string
	// starts son las líneas, ordenadas, donde empieza algo con posición
	// propia (elemento, ítem, fila): el campo que empieza en una termina
	// antes de la siguiente.
	starts []int
}

func (r *assetRebaser) collectStarts(doc *ast.AST) {
	seen := map[int]bool{}
	add := func(l int) {
		if l > 0 && !seen[l] {
			seen[l] = true
			r.starts = append(r.starts, l)
		}
	}
	var points func([]ast.PointItem)
	points = func(items []ast.PointItem) {
		for _, it := range items {
			add(it.Position.Line)
			points(it.SubPoints)
		}
	}
	var checks func([]ast.ChecklistItem)
	checks = func(items []ast.ChecklistItem) {
		for _, it := range items {
			add(it.Position.Line)
			checks(it.SubItems)
		}
	}
	_ = ast.Walk(doc, func(n ast.Node) error {
		if el, ok := n.(ast.Element); ok {
			add(el.GetPosition().Line)
		}
		switch e := n.(type) {
		case *ast.PointsElement:
			points(e.Items)
		case *ast.ChecklistElement:
			checks(e.Items)
		case *ast.TableElement:
			for _, p := range e.RowPositions {
				add(p.Line)
			}
		case *ast.GridElement:
			for _, c := range e.Columns {
				add(c.Position.Line)
			}
		}
		return nil
	})
	sort.Ints(r.starts)
}

// end devuelve la última línea que puede pertenecer a un campo que empieza
// en start: la anterior a la siguiente posición conocida, o el final.
func (r *assetRebaser) end(start int) int {
	i := sort.SearchInts(r.starts, start+1)
	if i < len(r.starts) {
		return r.starts[i] - 1
	}
	return len(r.lines)
}

func (r *assetRebaser) pointItems(items []ast.PointItem) {
	for i := range items {
		items[i].Content = r.inline(items[i].Content, items[i].Position.Line)
		r.pointItems(items[i].SubPoints)
	}
}

func (r *assetRebaser) checklistItems(items []ast.ChecklistItem) {
	for i := range items {
		items[i].Content = r.inline(items[i].Content, items[i].Position.Line)
		r.checklistItems(items[i].SubItems)
	}
}

// table usa la posición de cada fila (RowPositions) cuando la hay; los
// encabezados empiezan en la línea de la tabla. Cells, cuando es la forma
// derivada de Headers/Rows (una fila más que Rows), sigue la misma
// correspondencia; si no, todas sus filas parten de la línea de la tabla.
func (r *assetRebaser) table(e *ast.TableElement, line int) {
	rowLine := func(i int) int {
		if i < len(e.RowPositions) && e.RowPositions[i].Line > 0 {
			return e.RowPositions[i].Line
		}
		return line
	}
	f := r.newField(line)
	for i := range e.Headers {
		e.Headers[i] = f.inline(e.Headers[i])
	}
	for i, row := range e.Rows {
		f := r.newField(rowLine(i))
		for j := range row {
			row[j] = f.inline(row[j])
		}
	}
	derived := len(e.Cells) == len(e.Rows)+1
	for i, row := range e.Cells {
		l := line
		if derived && i > 0 {
			l = rowLine(i - 1)
		}
		f := r.newField(l)
		for j := range row {
			row[j].Content = f.inline(row[j].Content)
		}
	}
}

func (r *assetRebaser) realDir(dir string) string {
	if real, ok := r.realDirs[dir]; ok {
		return real
	}
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		real = dir
	}
	r.realDirs[dir] = real
	return real
}

// rebasePath reescribe src, escrito en un archivo de dir, relativo a
// sourceDir. ok es false cuando no hay nada que cambiar. Con destinos
// CommonMark src es una URL: se decodifica antes de unirla a dir y un "%"
// literal de la ruta nueva (una carpeta "100%") se vuelve a codificar, para
// que al leer el archivo se decodifique a la misma ruta. Los espacios quedan
// literales; quien la escribe decide cómo (<...> en un destino inline, %20 en
// un src).
func (r *assetRebaser) rebasePath(src, dir string) (string, bool) {
	if !isRelativeLocalSource(src) {
		return src, false
	}
	realDir := r.realDir(dir)
	if realDir == r.realSource {
		return src, false
	}
	path := src
	if commonMarkDestinations && strings.Contains(src, "%") {
		if decoded, err := url.PathUnescape(src); err == nil {
			path = decoded
		}
	}
	rel, err := filepath.Rel(r.realSource, filepath.Join(realDir, filepath.FromSlash(path)))
	if err != nil {
		return src, false
	}
	out := filepath.ToSlash(rel)
	if commonMarkDestinations {
		out = strings.ReplaceAll(out, "%", "%25")
	}
	return out, true
}

// field recorre, en orden, las líneas de un campo: cada fuente se busca a
// partir de donde se encontró la anterior.
type field struct {
	r         *assetRebaser
	line, col int
	end       int
}

func (r *assetRebaser) newField(start int) *field {
	return &field{r: r, line: start, end: r.end(start)}
}

// locate busca needle desde la posición actual y devuelve el directorio del
// archivo incluido del que viene la línea donde aparece, o "" si aparece en
// una línea del archivo raíz o no aparece.
func (f *field) locate(needle string) string {
	if needle == "" {
		return ""
	}
	for l := f.line; l >= 1 && l <= f.end && l <= len(f.r.lines); l++ {
		text := f.r.lines[l-1]
		from := 0
		if l == f.line {
			from = min(f.col, len(text))
		}
		if idx := strings.Index(text[from:], needle); idx >= 0 {
			f.line, f.col = l, from+idx+len(needle)
			if file := f.r.origins.File(l); file != "" {
				return filepath.Dir(file)
			}
			return ""
		}
	}
	return ""
}

// source reescribe la fuente de un campo del AST (imagen de bloque, video,
// póster).
func (f *field) source(src string) string {
	if !isRelativeLocalSource(src) {
		return src
	}
	dir := f.locate(src)
	if dir == "" {
		return src
	}
	out, _ := f.r.rebasePath(src, dir)
	return out
}

func (r *assetRebaser) inline(text string, line int) string {
	return r.newField(line).inline(text)
}

// inline reescribe el destino de cada ![alt](dest) de un texto Markdown
// inline, fuera de los code spans. dest es el destino crudo que reconoce el
// renderer (renderer.ScanLinkDestination): una ruta, opcionalmente entre
// <...> y seguida de un título; solo se cambia la ruta. Un destino con
// escapes (\) o referencias de caracteres (&) se deja como está, porque su
// ruta efectiva no es el texto literal.
func (f *field) inline(text string) string {
	if !strings.Contains(text, "![") {
		return text
	}
	var b strings.Builder
	i := 0
	for i < len(text) {
		switch {
		case text[i] == '`':
			end := generator.MarkdownCodeSpanEnd(text, i)
			b.WriteString(text[i:end])
			i = end
		case strings.HasPrefix(text[i:], "!["):
			m := inlineImageOpen.FindStringIndex(text[i:])
			if m == nil {
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
			b.WriteString(f.rebaseDestination(dest))
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

// commonMarkDestinations dice si el core enlazado lee los destinos como
// CommonMark (core > v2.49.0): una ruta suelta con espacios no es imagen,
// <...> delimita una ruta con espacios y las rutas locales son URLs que se
// decodifican (%20) al leer el archivo. Con un core anterior, una ruta con
// espacios se acepta suelta, <...> queda literal y nada se decodifica. Se
// pregunta al propio escáner de core para que doclang siga compilando y
// comportándose bien contra los dos.
var commonMarkDestinations = func() bool {
	_, _, ok := renderer.ScanLinkDestination("a b)")
	return !ok
}()

// rebaseDestination reescribe la ruta de un destino inline. Con destinos
// CommonMark, una ruta nueva con espacios (la carpeta del incluido los tiene)
// se escribe entre <...>, la única forma en que sigue siendo imagen. Con un
// core anterior se escribe tal cual, sin <...>, que es lo que resuelven su
// renderer HTML y el DOCX; la salida Markdown la envuelve en <...> al
// escribirla (ver generator.MarkdownImageDestinations).
func (f *field) rebaseDestination(dest string) string {
	if strings.ContainsAny(dest, `\&`) {
		return dest
	}
	dir := f.locate("(" + dest + ")")
	if dir == "" {
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
	} else if k := generator.MarkdownTitleStart(rest); k >= 0 {
		path, tail = rest[:k], rest[k:]
	} else {
		path = rest
	}
	newPath, ok := f.r.rebasePath(path, dir)
	if commonMarkDestinations && open == "" && strings.ContainsAny(newPath, " \t") {
		open, close = "<", ">"
	}
	if !ok || (open == "" && strings.ContainsAny(newPath, "()")) || (open != "" && strings.ContainsAny(newPath, "<>")) {
		return dest
	}
	return dest[:lead] + open + newPath + close + tail
}

// headingImgPattern es el <img> que el parser deja en el HTML de un
// encabezado flex con una imagen (`### Título ![h](h.png)`).
var headingImgPattern = regexp.MustCompile(`<img src="([^"]*)"`)

// headingHTML reescribe el src de cada <img> ya armado en el HTML de un
// encabezado. En el texto fuente la imagen sigue escrita como ![alt](src),
// así que se busca "(src" en sus líneas.
func (r *assetRebaser) headingHTML(text string, line int) string {
	if !strings.Contains(text, "<img src=\"") {
		return text
	}
	f := r.newField(line)
	return headingImgPattern.ReplaceAllStringFunc(text, func(tag string) string {
		src := headingImgPattern.FindStringSubmatch(tag)[1]
		if strings.Contains(src, "&") {
			return tag
		}
		dir := f.locate("(" + src)
		if dir == "" && commonMarkDestinations && strings.Contains(src, "%20") {
			// El src emitido lleva los espacios codificados; en la fuente
			// la ruta va entre <...> con los espacios literales.
			dir = f.locate("(<" + strings.ReplaceAll(src, "%20", " ") + ">")
		}
		if dir == "" {
			return tag
		}
		newPath, ok := r.rebasePath(src, dir)
		if !ok {
			return tag
		}
		if commonMarkDestinations {
			newPath = strings.NewReplacer(" ", "%20", "\t", "%09").Replace(newPath)
		}
		return `<img src="` + renderer.EscapeHTMLAttribute(newPath) + `"`
	})
}

func isRelativeLocalSource(src string) bool {
	if src == "" || strings.Contains(src, "{{") || strings.HasPrefix(src, "/") || strings.HasPrefix(src, "#") || filepath.IsAbs(src) {
		return false
	}
	u, err := url.Parse(src)
	return err == nil && u.Scheme == ""
}
