// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode"

	docx "github.com/mmonterroca/docxgo/v2"
	"github.com/mmonterroca/docxgo/v2/domain"
	"go.ziradocs.com/core/v2/a11y"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/renderer/chromium"
	"go.ziradocs.com/core/v2/util"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// DOCXGenerator genera documentos Word (.docx) usando docxgo v2
type DOCXGenerator struct {
	logger           util.Logger
	chromiumRenderer *chromium.ChromiumRenderer
	tempDir          string
	style            DOCXStyle
	// assetRoot confina las fuentes de imagen locales (elem.Source) a este
	// directorio. Ver docs/SECURITY_AUDIT_2026-07.md, AL-4.
	assetRoot string
}

// TOCEntry representa una entrada en la tabla de contenidos
type TOCEntry struct {
	Title      string
	Level      int // 1 = H1, 2 = H2, 3 = H3, 4 = H4
	BookmarkID string
}

// NewDOCXGenerator crea un nuevo generador DOCX. Si assetRoot está vacío
// (p. ej. un consumidor de la librería que construye GeneratorOptions{} sin
// configurarlo explícitamente), se confina al directorio de trabajo actual
// en vez de desactivar la confinación por completo — un opt-out silencioso
// habría revivido AL-4 para cualquier llamador que no pase por build.go
// (que sí siempre resuelve un AssetRoot concreto).
func NewDOCXGenerator(logger util.Logger, assetRoot string) *DOCXGenerator {
	if assetRoot == "" {
		if cwd, err := os.Getwd(); err == nil {
			assetRoot = cwd
		}
	}
	return &DOCXGenerator{
		logger:    logger,
		assetRoot: assetRoot,
	}
}

// Helper: Convertir twips string a int
func (g *DOCXGenerator) parseTwips(s string) int {
	var value int
	_, _ = fmt.Sscanf(s, "%d", &value)
	return value
}

// Helper: Convertir half-points string a int
func (g *DOCXGenerator) parseSize(s string) int {
	var value int
	_, _ = fmt.Sscanf(s, "%d", &value)
	return value
}

// Helper: Convertir hex string a domain.Color RGB
func (g *DOCXGenerator) parseColor(hex string) domain.Color {
	if len(hex) != 6 {
		return domain.Color{R: 0, G: 0, B: 0}
	}
	var r, gb, b uint8
	_, _ = fmt.Sscanf(hex, "%02x%02x%02x", &r, &gb, &b)
	return domain.Color{R: r, G: gb, B: b}
}

// Helper: Sanitizar string para bookmark ID
// bookmarkIDDisallowedChars es la whitelist inversa para sanitizeBookmarkID:
// cualquier carácter que no sea alfanumérico, "_" o "-" se elimina. El valor
// sanitizado se usa tanto como bookmark ID de OOXML como interpolado en un
// nombre de archivo temporal (mermaid_%s.png, chart_%s.png, map_%s.png) —
// la lista previa de reemplazos no cubría "/" ni "\", que sobrevivían intactos
// y podían redirigir esa escritura a un subdirectorio (ver
// docs/SECURITY_AUDIT_2026-07.md, BA-12).
var bookmarkIDDisallowedChars = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// accentTransliterator descompone runas acentuadas (NFD), descarta las
// marcas diacríticas combinantes (categoría Unicode Mn) y vuelve a componer
// (NFC) — dejando el carácter base más cercano: "á"→"a", "ñ"→"n", "Ü"→"U",
// etc. Es el idioma canónico de golang.org/x/text/runes para este trabajo
// (en vez de un loop manual repitiendo NFD+filtro Mn a mano), y no agrega
// una dependencia nueva: golang.org/x/text ya se sumó a go.mod para esto.
//
// Sin esto, sanitizeBookmarkID borraba directamente cualquier carácter
// fuera de la whitelist [A-Za-z0-9_-], incluyendo vocales acentuadas y
// "ñ"/"ü" (comunes en contenido en español, ver CLAUDE.md). Eso hacía que
// dos encabezados que sólo difirieran en acentos — p.ej. "Sección" vs
// "Seccion", "Publicación" vs "Publicacion" — sanitizaran al MISMO
// bookmark ID ("Seccion"/"Publicacion"), una colisión de bookmark ID
// inválida en OOXML si/cuando este campo se conecte a generación real de
// w:bookmarkStart (issues #112, #116). Transliterar antes de aplicar la
// whitelist no elimina toda posibilidad de colisión (un documento aún
// podría tener la versión acentuada y la no acentuada de la misma
// palabra), pero hace que las colisiones sean predecibles por contenido
// en vez de un accidente del orden de strip de caracteres.
var accentTransliterator = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

func transliterateToASCII(s string) string {
	result, _, err := transform.String(accentTransliterator, s)
	if err != nil {
		// No se espera que transform.String falle para NFD/NFC + un
		// filtro de runas — pero si pasara, es más seguro devolver el
		// string original (que igual pasará por la whitelist ASCII de
		// sanitizeBookmarkID) que perder el contenido por completo.
		return s
	}
	return result
}

func sanitizeBookmarkID(s string) string {
	s = transliterateToASCII(s)
	s = strings.ReplaceAll(s, " ", "_")
	return bookmarkIDDisallowedChars.ReplaceAllString(s, "")
}

// Generate genera un documento DOCX desde el AST
func (g *DOCXGenerator) Generate(astDoc *ast.AST, outputFile string, opts GeneratorOptions) error {
	g.logger.Info("DOCX", "Building DOCX document...")

	// Crear directorio temporal para imágenes renderizadas
	tempDir, err := os.MkdirTemp("", "doclang-docx-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()
	g.tempDir = tempDir

	// Inicializar tema del documento desde frontmatter
	themeName := "professional" // default
	if astDoc.FrontMatter != nil && astDoc.FrontMatter.Theme != "" {
		themeName = astDoc.FrontMatter.Theme
		g.logger.Info("DOCX", "Using theme from frontmatter: %s", themeName)
	}
	g.style = GetStyleForTheme(themeName, g.logger)

	// Inicializar ChromiumRenderer si hay elementos que requieren renderizado
	if g.needsChromiumRendering(astDoc) {
		g.logger.Info("DOCX", "Initializing Chromium renderer...")
		chromiumLogger := &renderer.ChromiumLoggerAdapter{Logger: g.logger}
		chromiumRenderer, err := chromium.NewChromiumRenderer(context.Background(), opts.ChromiumPath, opts.InstallChromium, chromiumLogger)
		if err != nil {
			g.logger.Warn("DOCX: Failed to initialize Chromium: %v", err)
		} else {
			g.chromiumRenderer = chromiumRenderer
			defer chromiumRenderer.Close()
		}
	}

	// Crear documento con docxgo v2
	doc := docx.NewDocument()

	// Configurar metadata
	if astDoc.FrontMatter != nil {
		meta := &domain.Metadata{}
		if astDoc.FrontMatter.Title != "" {
			meta.Title = astDoc.FrontMatter.Title
		}
		if astDoc.FrontMatter.Author != "" {
			meta.Creator = astDoc.FrontMatter.Author
		}
		_ = doc.SetMetadata(meta)

		// Idioma de revisión del documento (issue #62/#63 prerequisito): sin
		// esto, un .doclang con `lang: fr` producía un .docx sin idioma
		// declarado — Word usa el idioma del sistema para ortografía/lectura
		// de pantalla, no el del contenido. SetLanguage solo falla en un
		// documento abierto vía OpenDocument con styles.xml/settings.xml
		// preservados; docx.NewDocument() nunca cae en ese caso.
		//
		// Val/EastAsia/Bidi los tres al mismo valor (code review): Word solo
		// consulta Val para runs de script latino — EastAsia gobierna CJK y
		// Bidi gobierna RTL, y dejarlos sin poner (como antes) significaba
		// que un documento íntegramente en japonés o árabe con `lang: ja`/
		// `lang: ar` seguía corrigiéndose con el idioma del sistema en esos
		// runs, justo el defecto que este cambio existe para arreglar. El
		// AST solo tiene UN idioma declarado por documento (no por script),
		// así que replicarlo en los tres slots es lo mejor que se puede
		// hacer sin un campo de idioma por-run (issue #63, aparte); para un
		// documento de un solo script no cambia nada — Word solo aplica el
		// slot que corresponde al script real presente.
		if astDoc.FrontMatter.Lang != "" {
			lang := astDoc.FrontMatter.Lang
			if err := doc.SetLanguage(&domain.Language{Val: lang, EastAsia: lang, Bidi: lang}); err != nil {
				g.logger.Warn("DOCX: failed to set document language %q: %v", lang, err)
			}
		}
	}

	// Renderizar frontmatter (título del documento)
	if err := g.renderFrontMatter(doc, astDoc.FrontMatter); err != nil {
		return fmt.Errorf("error rendering frontmatter: %w", err)
	}

	// header:/footer: nativos de Word (issue #117): a diferencia de HTML/PDF,
	// DOCX nunca tuvo chrome propio, así que opts.HeaderFooter == nil no
	// necesita ningún fallback legado.
	if err := g.renderHeaderFooter(doc, opts.HeaderFooter, astDoc.FrontMatter.BuildVariables()); err != nil {
		return fmt.Errorf("error rendering header/footer: %w", err)
	}

	// Renderizar TOC (opts.TOC, issue #115 follow-up: antes se emitía
	// incondicionalmente, así que `--toc=false`/`toc: false` no tenían
	// ningún efecto en DOCX).
	if opts.TOC {
		tocEntries := g.collectHeadings(astDoc, opts.Numbering)
		if err := g.renderTOC(doc, tocEntries); err != nil {
			return fmt.Errorf("error rendering TOC: %w", err)
		}
	}

	// Renderizar secciones del documento. sectionNum replica el loop de
	// markdown.go's Generate (mismo resolveSectionTitle compartido): el
	// contador solo avanza en bloques "numbered" (los que vinieron de
	// block.Title, no del preámbulo con block.Heading), así que el
	// preámbulo nunca corre la numeración del resto — el bug que #100
	// arregló para HTML/Markdown y que DOCX nunca tuvo hasta ahora.
	sectionNum := 1
	for i := range astDoc.ContentBlocks {
		block := &astDoc.ContentBlocks[i]
		title, numbered := resolveSectionTitle(*block)
		num := 0
		if opts.Numbering && numbered {
			num = sectionNum
		}
		if err := g.renderSection(doc, block, title, num); err != nil {
			return fmt.Errorf("error rendering section: %w", err)
		}
		if numbered {
			sectionNum++
		}

		// Page break entre secciones (opts.PageBreaks), mismo punto del
		// loop que HTML/Markdown: nunca después de la última sección.
		if opts.PageBreaks && i < len(astDoc.ContentBlocks)-1 {
			if err := doc.AddPageBreak(); err != nil {
				return fmt.Errorf("error adding page break: %w", err)
			}
		}
	}

	// Guardar documento
	if err := doc.SaveAs(outputFile); err != nil {
		return fmt.Errorf("failed to save DOCX: %w", err)
	}

	g.logger.Info("DOCX", "✅ DOCX document generated successfully")
	return nil
}

// renderFrontMatter renderiza el título, autor y fecha del documento
func (g *DOCXGenerator) renderFrontMatter(doc domain.Document, fm *ast.FrontMatterNode) error {
	if fm == nil {
		return nil
	}

	// Título principal
	if fm.Title != "" {
		p, err := doc.AddParagraph()
		if err != nil {
			return err
		}
		if err := p.SetSpacingBefore(g.parseTwips(g.style.H1SpaceBefore)); err != nil {
			return fmt.Errorf("invalid spacing before: %w", err)
		}
		if err := p.SetSpacingAfter(g.parseTwips(g.style.H1SpaceAfter)); err != nil {
			return fmt.Errorf("invalid spacing after: %w", err)
		}
		if err := p.SetAlignment(domain.AlignmentCenter); err != nil {
			return fmt.Errorf("invalid alignment: %w", err)
		}

		r, err := p.AddRun()
		if err != nil {
			return err
		}
		_ = r.SetText(fm.Title)
		if err := r.SetSize(g.parseSize(g.style.H1Size)); err != nil {
			return fmt.Errorf("invalid font size: %w", err)
		}
		_ = r.SetColor(g.parseColor(g.style.H1Color))
		_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
		if g.style.H1Bold {
			_ = r.SetBold(true)
		}
	}

	// Autor
	if fm.Author != "" {
		p, err := doc.AddParagraph()
		if err != nil {
			return err
		}
		if err := p.SetSpacingAfter(g.parseTwips(g.style.TextSpaceAfter)); err != nil {
			return fmt.Errorf("invalid spacing after: %w", err)
		}
		if err := p.SetAlignment(domain.AlignmentCenter); err != nil {
			return fmt.Errorf("invalid alignment: %w", err)
		}

		r, err := p.AddRun()
		if err != nil {
			return err
		}
		_ = r.SetText("Por: " + fm.Author)
		if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
			return fmt.Errorf("invalid font size: %w", err)
		}
		_ = r.SetColor(g.parseColor(g.style.TextLightColor))
		_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
	}

	// Fecha
	if fm.Date != "" {
		p, err := doc.AddParagraph()
		if err != nil {
			return err
		}
		if err := p.SetSpacingAfter(g.parseTwips(g.style.TextSpaceAfter)); err != nil {
			return fmt.Errorf("invalid spacing after: %w", err)
		}
		if err := p.SetAlignment(domain.AlignmentCenter); err != nil {
			return fmt.Errorf("invalid alignment: %w", err)
		}

		r, err := p.AddRun()
		if err != nil {
			return err
		}
		_ = r.SetText(fm.Date)
		if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
			return fmt.Errorf("invalid font size: %w", err)
		}
		_ = r.SetColor(g.parseColor(g.style.TextLightColor))
		_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
	}

	// Espacio después del frontmatter
	if fm.Title != "" || fm.Author != "" || fm.Date != "" {
		p, err := doc.AddParagraph()
		if err != nil {
			return err
		}
		if err := p.SetSpacingAfter(240); err != nil {
			return fmt.Errorf("invalid spacing after: %w", err)
		}
	}

	return nil
}

// docxParagraphAdder es el subconjunto común de domain.Header/domain.Footer
// que renderHeaderFooterZones necesita — ambas interfaces solo exponen
// AddParagraph()/Paragraphs(), sin un tipo compartido en docxgo, así que
// esta interfaz local deja pasar cualquiera de las dos sin duplicar la
// función por header/footer.
type docxParagraphAdder interface {
	AddParagraph() (domain.Paragraph, error)
}

// renderHeaderFooter escribe el header/footer nativo de Word (issue #117)
// vía section.Header(domain.HeaderDefault)/Footer(domain.FooterDefault).
// hf == nil (nunca hubo `header:`/`footer:` en el front matter) no escribe
// nada — a diferencia de HTML/PDF, DOCX nunca tuvo chrome propio, así que
// no hay ningún fallback legado que preservar acá.
//
// Solo Text y PageNumbers se traducen a este backend: Logo/Border/Height/
// Background no tienen equivalente en la superficie que expone
// domain.Section (PageSize/Margins/Orientation/Columns/Header/Footer, ver
// github.com/mmonterroca/docxgo/v2/domain/section.go) — no hay una API de
// bordes o imágenes de página que envolver, así que se omiten en vez de
// fingir soportarlos.
//
// layout_defaults tampoco aplica: un section.Header/Footer de Word es
// global para toda la sección (no hay noción de "el bloque que abre esta
// página" al momento de generar el documento, igual que en el backend de
// PDF — ver pdfChromeZonesHTML), así que solo se lee el header/footer
// GLOBAL del front matter.
func (g *DOCXGenerator) renderHeaderFooter(doc domain.Document, hf *ast.HeaderFooterConfig, variables map[string]interface{}) error {
	if hf == nil {
		return nil
	}

	section, err := doc.DefaultSection()
	if err != nil {
		return fmt.Errorf("failed to get default section: %w", err)
	}

	if hf.Header != nil && hf.Header.Enabled {
		header, err := section.Header(domain.HeaderDefault)
		if err != nil {
			return fmt.Errorf("failed to get header: %w", err)
		}
		if err := renderHeaderFooterZones(header, hf.Header.Text, variables); err != nil {
			return fmt.Errorf("failed to render header text: %w", err)
		}
	}

	if hf.Footer != nil && hf.Footer.Enabled {
		footer, err := section.Footer(domain.FooterDefault)
		if err != nil {
			return fmt.Errorf("failed to get footer: %w", err)
		}
		if err := renderHeaderFooterZones(footer, hf.Footer.Text, variables); err != nil {
			return fmt.Errorf("failed to render footer text: %w", err)
		}
		if hf.Footer.PageNumbers != nil && hf.Footer.PageNumbers.Enabled {
			if err := renderFooterPageNumbers(footer, hf.Footer.PageNumbers, variables); err != nil {
				return fmt.Errorf("failed to render footer page numbers: %w", err)
			}
		}
	}

	return nil
}

// renderHeaderFooterZones escribe cada zona no vacía (Left/Center/Right)
// como su propio párrafo alineado. docxgo no expone una API de tab stops
// (ver domain/paragraph.go) — sin eso no hay forma de poner las tres zonas
// en una sola línea como hacen HTML/PDF, así que cada una ocupa su propia
// línea; limitación documentada del backend DOCX, no un recorte de este PR.
func renderHeaderFooterZones(target docxParagraphAdder, text *ast.HeaderFooterText, variables map[string]interface{}) error {
	if text == nil {
		return nil
	}

	zones := []struct {
		value string
		align domain.Alignment
	}{
		{text.Left, domain.AlignmentLeft},
		{text.Center, domain.AlignmentCenter},
		{text.Right, domain.AlignmentRight},
	}

	for _, zone := range zones {
		processed := renderer.ProcessVariables(zone.value, variables)
		if processed == "" {
			continue
		}
		para, err := target.AddParagraph()
		if err != nil {
			return err
		}
		if err := para.SetAlignment(zone.align); err != nil {
			return err
		}
		run, err := para.AddRun()
		if err != nil {
			return err
		}
		if err := run.AddText(processed); err != nil {
			return err
		}
	}

	return nil
}

// renderFooterPageNumbers escribe un párrafo de numeración de página en el
// footer usando los campos nativos de Word (docx.NewPageNumberField/
// NewPageCountField) — a diferencia de HTML/PDF, este es el único backend
// donde {{current}}/{{total}} pueden mostrar la paginación REAL del
// documento (Word la recalcula al abrir/imprimir), no un conteo de
// ContentBlocks. Ver splitPageNumberFormat para la tokenización del
// formato.
//
// variables resuelve las variables del documento del autor dentro de
// Format (p. ej. `{{company}}`) ANTES de tokenizar — code review sobre
// este PR: Format es texto de front matter como cualquier otro, así que
// debe pasar por el mismo ProcessVariables que renderHeaderFooterZones ya
// aplica a header.text/footer.text. splitPageNumberFormat solo tokeniza
// literalmente "{{current}}"/"{{total}}", que ProcessVariables deja
// intactos porque no son claves del mapa de variables del documento.
func renderFooterPageNumbers(footer domain.Footer, config *ast.PageNumbersConfig, variables map[string]interface{}) error {
	format := config.Format
	if format == "" {
		format = "{{current}} / {{total}}"
	}
	format = renderer.ProcessVariables(format, variables)

	para, err := footer.AddParagraph()
	if err != nil {
		return err
	}
	align := domain.AlignmentRight
	switch config.Position {
	case "left":
		align = domain.AlignmentLeft
	case "center":
		align = domain.AlignmentCenter
	}
	if err := para.SetAlignment(align); err != nil {
		return err
	}

	for _, part := range splitPageNumberFormat(format) {
		run, err := para.AddRun()
		if err != nil {
			return err
		}
		switch part.kind {
		case pageNumberPartText:
			if err := run.AddText(part.text); err != nil {
				return err
			}
		case pageNumberPartCurrent:
			if err := run.AddField(docx.NewPageNumberField()); err != nil {
				return err
			}
		case pageNumberPartTotal:
			if err := run.AddField(docx.NewPageCountField()); err != nil {
				return err
			}
		}
		switch config.Style {
		case "bold":
			_ = run.SetBold(true)
		case "caption":
			_ = run.SetItalic(true)
		}
	}

	return nil
}

type pageNumberPartKind int

const (
	pageNumberPartText pageNumberPartKind = iota
	pageNumberPartCurrent
	pageNumberPartTotal
)

type pageNumberPart struct {
	kind pageNumberPartKind
	text string
}

// splitPageNumberFormat tokeniza un PageNumbersConfig.Format en runs de
// texto literal intercalados con los tokens {{current}}/{{total}} — mismo
// contrato de sustitución que documentan core/renderer/document_html.go
// (renderFooterPageNumbers) y el backend de PDF de este paquete
// (pdfPageNumberHTML), pero acá cada token se materializa como un campo
// nativo de Word en vez de texto o una clase especial de Chromium.
func splitPageNumberFormat(format string) []pageNumberPart {
	const (
		tokenCurrent = "{{current}}"
		tokenTotal   = "{{total}}"
	)

	var parts []pageNumberPart
	rest := format
	for rest != "" {
		iCurrent := strings.Index(rest, tokenCurrent)
		iTotal := strings.Index(rest, tokenTotal)

		next := -1
		var kind pageNumberPartKind
		var tokenLen int
		switch {
		case iCurrent != -1 && (iTotal == -1 || iCurrent < iTotal):
			next, kind, tokenLen = iCurrent, pageNumberPartCurrent, len(tokenCurrent)
		case iTotal != -1:
			next, kind, tokenLen = iTotal, pageNumberPartTotal, len(tokenTotal)
		}

		if next == -1 {
			parts = append(parts, pageNumberPart{kind: pageNumberPartText, text: rest})
			break
		}
		if next > 0 {
			parts = append(parts, pageNumberPart{kind: pageNumberPartText, text: rest[:next]})
		}
		parts = append(parts, pageNumberPart{kind: kind})
		rest = rest[next+tokenLen:]
	}

	return parts
}

// gridColumnHeading{HTML,MD}{2,3,4}Pattern replican exactamente los patrones
// que renderText usa (en orden: HTML primero, luego Markdown) para decidir
// si una línea de columna de grid se renderiza con un estilo Heading2/3/4
// real de Word. Deben permanecer en sync con renderText — de lo contrario
// el TOC estático (collectHeadings) y el campo TOC real de Word (que
// autodetecta cualquier párrafo Heading1-3 al refrescar el campo con F9)
// divergen (issue #88).
var (
	gridColumnHeadingHTML2Pattern = regexp.MustCompile(`^<h2[^>]*>(.+?)</h2>`)
	gridColumnHeadingHTML3Pattern = regexp.MustCompile(`^<h3[^>]*>(.+?)</h3>`)
	gridColumnHeadingHTML4Pattern = regexp.MustCompile(`^<h4[^>]*>(.+?)</h4>`)
	gridColumnHeadingMD2Pattern   = regexp.MustCompile(`^## (.+)`)
	gridColumnHeadingMD3Pattern   = regexp.MustCompile(`^### (.+)`)
	gridColumnHeadingMD4Pattern   = regexp.MustCompile(`^#### (.+)`)

	// gridColumnHeadingPatterns ordena los patrones anteriores (HTML antes
	// que Markdown, igual que renderText) para poder recorrerlos en un
	// loop en vez de un if/else-if de 6 ramas casi idénticas.
	gridColumnHeadingPatterns = []struct {
		pattern *regexp.Regexp
		level   int
	}{
		{gridColumnHeadingHTML2Pattern, 2},
		{gridColumnHeadingHTML3Pattern, 3},
		{gridColumnHeadingHTML4Pattern, 4},
		{gridColumnHeadingMD2Pattern, 2},
		{gridColumnHeadingMD3Pattern, 3},
		{gridColumnHeadingMD4Pattern, 4},
	}
)

// collectHeadings recolecta todos los encabezados del documento para el TOC.
// El H1 de cada bloque usa resolveSectionTitle y el mismo contador
// sectionNum que el loop de sección en Generate (issue #115 follow-up code
// review: antes leía block.Title directo, así que el preámbulo —cuyo texto
// vive en block.Heading, ver resolveSectionTitle— quedaba fuera del TOC, y
// el prefijo numérico del TOC podía divergir del que realmente escribe
// renderSection en el cuerpo cuando opts.Numbering estaba activo).
func (g *DOCXGenerator) collectHeadings(astDoc *ast.AST, numbering bool) []TOCEntry {
	var entries []TOCEntry

	g.logger.Info("DOCX", "📋 Collecting headings from %d content blocks", len(astDoc.ContentBlocks))

	sectionNum := 1
	for _, block := range astDoc.ContentBlocks {
		// H1 (section title, o Heading si es el bloque de preámbulo)
		title, numbered := resolveSectionTitle(block)
		if title != "" {
			entryTitle := title
			if numbering && numbered {
				entryTitle = fmt.Sprintf("%d. %s", sectionNum, title)
			}
			entries = append(entries, TOCEntry{
				Title:      entryTitle,
				Level:      1,
				BookmarkID: sanitizeBookmarkID(title),
			})
			g.logger.Info("DOCX", "  ➜ H1: %s", entryTitle)
		}
		if numbered {
			sectionNum++
		}

		// H1-H6 (text elements with raw HTML or markdown headers)
		for _, elem := range block.Elements {
			switch typedElem := elem.(type) {
			case *ast.TextElement:
				content := typedElem.Content

				// Level (issue #22) es la fuente de verdad cuando está
				// poblado — mismo criterio que renderText/renderHeading,
				// que hoy cubren niveles 1 y 5/6 además de 2/3/4. Sin este
				// caso, un heading de nivel 5/6 (o un nivel 1 que llegó vía
				// --filter externo, no del título de bloque) se renderiza
				// como un heading real de Word pero nunca aparece en este
				// TOC estático, porque el bloque de regexes de abajo solo
				// reconoce h2/h3/h4 (issue detectado en code review de #40).
				if typedElem.Level > 0 {
					title := content
					if m := headingHTMLPattern.FindStringSubmatch(content); m != nil {
						title = m[1]
					} else if m := markdownHeadingPattern.FindStringSubmatch(content); m != nil {
						title = m[1]
					}
					// docxStripHeadingMarkup (issue #63 code review finding
					// #1): title puede traer markup ya renderizado
					// (<span lang="xx">, <strong>, ...) — el TOC estático
					// solo escribe texto plano (entryRun.SetText más abajo)
					// y sanitizeBookmarkID necesita texto plano para no
					// corromper el bookmark ID con el markup.
					title = docxStripHeadingMarkup(title)
					entries = append(entries, TOCEntry{
						Title:      title,
						Level:      typedElem.Level,
						BookmarkID: sanitizeBookmarkID(title),
					})
					g.logger.Info("DOCX", "  ➜ H%d: %s", typedElem.Level, title)
					continue
				}

				// Check for raw HTML headers (IsRawHTML=true)
				//
				// docxStripHeadingMarkup en las 6 ramas de abajo (issue #63
				// code review finding #1, mismo criterio que la rama
				// Level > 0 arriba): el grupo capturado puede traer markup
				// ya renderizado (<span lang="xx">, <strong>, ...) que
				// corrompería el TOC estático y sanitizeBookmarkID si se
				// usara tal cual.
				if typedElem.IsRawHTML {
					if h2Match := regexp.MustCompile(`<h2[^>]*>(.+?)</h2>`).FindStringSubmatch(content); h2Match != nil {
						title := docxStripHeadingMarkup(h2Match[1])
						entries = append(entries, TOCEntry{
							Title:      title,
							Level:      2,
							BookmarkID: sanitizeBookmarkID(title),
						})
						g.logger.Info("DOCX", "  ➜ H2: %s", title)
					} else if h3Match := regexp.MustCompile(`<h3[^>]*>(.+?)</h3>`).FindStringSubmatch(content); h3Match != nil {
						title := docxStripHeadingMarkup(h3Match[1])
						entries = append(entries, TOCEntry{
							Title:      title,
							Level:      3,
							BookmarkID: sanitizeBookmarkID(title),
						})
						g.logger.Info("DOCX", "  ➜ H3: %s", title)
					} else if h4Match := regexp.MustCompile(`<h4[^>]*>(.+?)</h4>`).FindStringSubmatch(content); h4Match != nil {
						title := docxStripHeadingMarkup(h4Match[1])
						entries = append(entries, TOCEntry{
							Title:      title,
							Level:      4,
							BookmarkID: sanitizeBookmarkID(title),
						})
						g.logger.Info("DOCX", "  ➜ H4: %s", title)
					}
				} else {
					// Check for markdown headers (## text)
					if h2Match := regexp.MustCompile(`^## (.+)`).FindStringSubmatch(content); h2Match != nil {
						title := docxStripHeadingMarkup(h2Match[1])
						entries = append(entries, TOCEntry{
							Title:      title,
							Level:      2,
							BookmarkID: sanitizeBookmarkID(title),
						})
						g.logger.Info("DOCX", "  ➜ H2: %s", title)
					} else if h3Match := regexp.MustCompile(`^### (.+)`).FindStringSubmatch(content); h3Match != nil {
						title := docxStripHeadingMarkup(h3Match[1])
						entries = append(entries, TOCEntry{
							Title:      title,
							Level:      3,
							BookmarkID: sanitizeBookmarkID(title),
						})
						g.logger.Info("DOCX", "  ➜ H3: %s", title)
					} else if h4Match := regexp.MustCompile(`^#### (.+)`).FindStringSubmatch(content); h4Match != nil {
						title := docxStripHeadingMarkup(h4Match[1])
						entries = append(entries, TOCEntry{
							Title:      title,
							Level:      4,
							BookmarkID: sanitizeBookmarkID(title),
						})
						g.logger.Info("DOCX", "  ➜ H4: %s", title)
					}
				}

			case *ast.GridElement:
				// renderGrid (ver más abajo) renderiza column.Content
				// línea por línea vía renderText, que sí crea estilos de
				// heading reales de Word para líneas "## "/"### "/"#### "
				// (o sus equivalentes HTML) dentro de una columna de grid
				// (issue #56). collectHeadings nunca recorría
				// GridElement.Columns, así que esos headings quedaban
				// fuera del TOC estático aunque el campo TOC real de Word
				// sí los detecta al refrescarse — issue #88.
				for _, column := range typedElem.Columns {
					for _, line := range strings.Split(column.Content, "\n") {
						// renderGrid (más abajo) solo usa TrimSpace para
						// decidir si una línea está en blanco — el texto
						// que realmente le pasa a renderText es la línea
						// SIN recortar, y los patrones de heading de
						// renderText están anclados con "^" (no toleran
						// espacio inicial). Si aquí se hiciera el match
						// contra la línea recortada, una línea de columna
						// indentada (parseColumn preserva la indentación
						// original, ver core/elements/grid.go)
						// se agregaría al TOC como heading aunque
						// renderGrid la renderice como párrafo plano —
						// exactamente la divergencia TOC/render que este
						// fix busca eliminar (issue #88). Por eso se hace
						// match contra `line` tal cual, no contra su
						// versión recortada.
						if strings.TrimSpace(line) == "" {
							continue
						}

						for _, hp := range gridColumnHeadingPatterns {
							m := hp.pattern.FindStringSubmatch(line)
							if m == nil {
								continue
							}
							// docxStripHeadingMarkup (issue #63 code review
							// finding #1): m[1] puede traer markup ya
							// renderizado cuando vino de un
							// gridColumnHeadingHTML*Pattern.
							title := docxStripHeadingMarkup(m[1])
							entries = append(entries, TOCEntry{
								Title:      title,
								Level:      hp.level,
								BookmarkID: sanitizeBookmarkID(title),
							})
							g.logger.Info("DOCX", "  ➜ H%d (grid column): %s", hp.level, title)
							break
						}
					}
				}
			}
		}
	}

	g.logger.Info("DOCX", "📋 Found %d TOC entries", len(entries))
	return entries
}

// renderTOC renderiza la tabla de contenidos (placeholder por ahora, TOC real después)
func (g *DOCXGenerator) renderTOC(doc domain.Document, entries []TOCEntry) error {
	if len(entries) == 0 {
		return nil
	}

	// Título "Tabla de Contenidos"
	p, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := p.SetSpacingBefore(240); err != nil {
		return fmt.Errorf("invalid spacing before: %w", err)
	}
	if err := p.SetSpacingAfter(120); err != nil {
		return fmt.Errorf("invalid spacing after: %w", err)
	}

	r, err := p.AddRun()
	if err != nil {
		return err
	}
	_ = r.SetText("Tabla de Contenidos")
	if err := r.SetSize(g.parseSize(g.style.H2Size)); err != nil {
		return fmt.Errorf("invalid font size: %w", err)
	}
	_ = r.SetColor(g.parseColor(g.style.H2Color))
	_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
	_ = r.SetBold(true)

	// Campo de TOC real (Word generará el TOC automáticamente)
	tocPara, err := doc.AddParagraph()
	if err != nil {
		return err
	}

	// Crear run para el field
	tocRun, err := tocPara.AddRun()
	if err != nil {
		return err
	}

	// Crear field TOC usando el constructor con switches. OJO: docxgo's
	// NewTOCField/buildTOCCode (internal/core/field.go en el módulo
	// vendido) NO lee las claves "o"/"h"/"z"/"u" que este código tenía
	// antes (los caracteres del switch de Word) — lee "levels" para el
	// \o, y \h/\z/\u salen SIEMPRE hardcodeados sin importar qué se pase.
	// Un mapa con clave "o" es un no-op silencioso: el field generado
	// siempre terminaba con el default "1-3" de la librería, sin importar
	// qué dijera este mapa — confirmado corriendo el generador real y
	// grepeando el \o del document.xml resultante. "levels" es la única
	// clave que buildTOCCode efectivamente consulta.
	//
	// El valor en sí, "1-4": H1-H3 mapean 1:1 a outline 1-3, pero Level 4
	// Y TAMBIÉN 5/6 (degradados a StyleIDHeading4 en renderHeading, sin
	// estilo H5/H6 propio) usan el estilo Heading4 = outline 4. Si se
	// quedara en el default "1-3" de la librería, Word ocultaría esos
	// headings del TOC real en cuanto el usuario lo actualice (clic
	// derecho → Actualizar campo, o F9) o abra el documento, aunque el
	// placeholder estático (collectHeadings) sí los liste — las dos
	// vistas del TOC divergirían entre sí.
	tocSwitches := map[string]string{
		"levels": "1-4",
	}
	tocField := docx.NewTOCField(tocSwitches)

	// Agregar el field al run
	err = tocRun.AddField(tocField)
	if err != nil {
		return err
	}

	// Generar contenido placeholder para cuando Word aún no ha actualizado
	// (Word reemplazará esto al abrir el documento o presionar F9)
	g.logger.Info("DOCX", "📑 TOC field added - press F9 in Word to update")
	for _, entry := range entries {
		entryPara, err := doc.AddParagraph()
		if err != nil {
			return err
		}

		indent := (entry.Level - 1) * 360 // 360 twips = 0.25" por nivel
		if err := entryPara.SetIndent(domain.Indentation{Left: indent}); err != nil {
			return fmt.Errorf("invalid indent: %w", err)
		}

		entryRun, err := entryPara.AddRun()
		if err != nil {
			return err
		}
		_ = entryRun.SetText(entry.Title)
		if err := entryRun.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
			return fmt.Errorf("invalid font size: %w", err)
		}
		_ = entryRun.SetFont(domain.Font{Name: g.style.FontFamily})
		_ = entryRun.SetColor(domain.Color{R: 128, G: 128, B: 128}) // Gris para placeholder
	}

	// Espacio después del TOC
	spacer, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := spacer.SetSpacingAfter(240); err != nil {
		return fmt.Errorf("invalid spacing after: %w", err)
	}

	return nil
}

// renderSection renderiza una sección del documento (H1 + elementos). title
// y num vienen ya resueltos por el caller (resolveSectionTitle + el contador
// de sectionNum de Generate) en vez de leer section.Title directamente: eso
// es lo que hace que un ContentBlock de preámbulo (Title vacío, Heading
// poblado — ver resolveSectionTitle) también reciba su encabezado, igual que
// ya hacían los generadores HTML y Markdown; num == 0 significa "sin
// numerar" (numbering deshabilitado, o esta sección no numera — el
// preámbulo).
func (g *DOCXGenerator) renderSection(doc domain.Document, section *ast.ContentBlock, title string, num int) error {
	// Título de la sección (H1)
	if title != "" {
		p, err := doc.AddParagraph()
		if err != nil {
			return err
		}

		// Estilo nativo Heading1
		_ = p.SetStyle(domain.StyleIDHeading1)
		if err := p.SetSpacingBefore(g.parseTwips(g.style.H1SpaceBefore)); err != nil {
			return fmt.Errorf("invalid spacing before: %w", err)
		}
		if err := p.SetSpacingAfter(g.parseTwips(g.style.H1SpaceAfter)); err != nil {
			return fmt.Errorf("invalid spacing after: %w", err)
		}

		text := title
		if num > 0 {
			text = fmt.Sprintf("%d. %s", num, title)
		}

		r, err := p.AddRun()
		if err != nil {
			return err
		}
		_ = r.SetText(text)
		if err := r.SetSize(g.parseSize(g.style.H1Size)); err != nil {
			return fmt.Errorf("invalid font size: %w", err)
		}
		_ = r.SetColor(g.parseColor(g.style.H1Color))
		_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
		_ = r.SetBold(g.style.H1Bold)

		// TODO: Agregar bookmark para TOC
	}

	// Renderizar elementos de la sección
	for _, elem := range section.Elements {
		if err := g.renderElement(doc, elem); err != nil {
			return err
		}
	}

	// Espacio entre secciones
	p, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	_ = p // Párrafo vacío

	return nil
}

// renderElement dispatcher para diferentes tipos de elementos
func (g *DOCXGenerator) renderElement(doc domain.Document, elem ast.Element) error {
	switch e := elem.(type) {
	case *ast.TextElement:
		return g.renderText(doc, e)
	case *ast.PointsElement:
		return g.renderPoints(doc, e)
	case *ast.CodeElement:
		return g.renderCode(doc, e)
	case *ast.TableElement:
		return g.renderTable(doc, e)
	case *ast.ImageElement:
		return g.renderImage(doc, e)
	case *ast.MediaElement:
		return g.renderMedia(doc, e)
	case *ast.MermaidElement:
		return g.renderMermaid(doc, e)
	case *ast.ChartElement:
		return g.renderChart(doc, e)
	case *ast.MapElement:
		return g.renderMap(doc, e)
	case *ast.QuoteElement:
		return g.renderQuote(doc, e)
	case *ast.ChecklistElement:
		return g.renderChecklist(doc, e)
	case *ast.SpecialBlockElement:
		return g.renderSpecialBlock(doc, e)
	case *ast.CodeGroupElement:
		return g.renderCodeGroup(doc, e)
	case *ast.PlantUMLElement:
		return g.renderPlantUML(doc, e)
	case *ast.GridElement:
		return g.renderGrid(doc, e)
	case *ast.MathElement:
		return g.renderMath(doc, e)
	case *ast.DirectiveNode:
		// Same rationale as markdown.go's case: a directive (@notes,
		// @timer, …) is slidelang presenter-notes metadata with no
		// document equivalent, so its content is not rendered — but the
		// warning names it instead of falling through to the generic
		// "Unknown element type", which would misleadingly imply the type
		// itself is unrecognized.
		g.logger.Warn("DOCX: la directiva @%s (línea %d) no tiene efecto en un documento y se omite; usá un blockquote si querés que su contenido se vea",
			e.Name, e.GetPosition().Line)
		return nil
	default:
		g.logger.Warn("DOCX: Unknown element type: %T", elem)
		return nil
	}
}

// renderText renderiza texto/párrafos/headings
// headingHTMLPattern extrae el texto interno de un <hN id="...">texto</hN>
// ya renderizado — usado solo cuando Level (issue #22) YA nos dijo qué
// nivel es; a diferencia del bloque de regexes de abajo, esto no ADIVINA el
// nivel probando h2/h3/h4 en secuencia, solo despoja el wrapper del payload
// cuyo nivel ya conocemos. `\s*$` en vez de `$` a secas: los seis regexes
// que reemplaza más abajo no tenían ningún anchor final, y una versión
// anclada con `$` no toleraba contenido/espacio colgante después del
// `</hN>` (p. ej. un salto de línea), lo que hacía que no matcheara nada y
// se renderizara la marca cruda como texto del heading. Pero un `$` sin
// `\s*` delante tampoco sirve: sin ÉL, Content que combina un heading con
// contenido real después (`<h2>Título</h2><p>Contenido importante</p>`,
// alcanzable vía un --filter externo o un TextElement mal formado) SÍ
// matchea igual — el heading matchea como prefijo del string y el resto
// de Content (el párrafo) se pierde en silencio, sin ni siquiera quedar
// como texto crudo. `\s*$` exige que después del cierre solo quede
// whitespace: tolera el salto de línea colgante (el caso real que este
// pattern debe cubrir) pero rechaza contenido genuino, que entonces cae
// al fallback de "texto crudo" (visible, no perdido).
var headingHTMLPattern = regexp.MustCompile(`^<h[0-9]+[^>]*>(.+?)</h[0-9]+>\s*$`)

// markdownHeadingPattern despoja un prefijo Markdown `#`..`######` — el
// sibling de headingHTMLPattern para Content que llegó como Markdown crudo
// en vez de HTML ya renderizado (un AST externo vía --filter, o un
// TextElement cuyo Level se pobló antes de que corriera la normalización
// HTML). `\s*$`, misma razón que headingHTMLPattern: `.` no matchea `\n`
// en Go regexp, así que un Content de dos líneas ("## Título\nContenido")
// ya paraba de capturar en el salto de línea sin necesidad del anchor —
// pero sin `\s*$` esa segunda línea igual se perdía en silencio (el
// código solo usa el grupo capturado, descarta el resto de Content). Con
// `\s*$`, ese caso deja de matchear del todo y cae al fallback de texto
// crudo en vez de perder la línea.
var markdownHeadingPattern = regexp.MustCompile(`^#{1,6}\s+(.+?)\s*$`)

// headingIDPattern extrae el id="..." de un <hN id="...">...</hN> ya
// renderizado, cuando existe — usado por el generador de Markdown para
// preservar el anchor explícito (ver markdown.go).
var headingIDPattern = regexp.MustCompile(`^<h[0-9]+[^>]*\bid="([^"]*)"`)

func (g *DOCXGenerator) renderText(doc domain.Document, elem *ast.TextElement) error {
	content := elem.Content

	// Level (issue #22) es la fuente de verdad cuando está poblado — evita
	// el acoplamiento frágil de re-derivar el NIVEL re-parseando el <hN> ya
	// renderizado, que es justo lo que Level se agregó para eliminar (ver
	// core/ast/nodes.go). Level == 0 cubre DOS casos legítimos, no solo
	// "dato faltante": un TextElement que simplemente no es un heading
	// declarado (un párrafo normal — la inmensa mayoría), y un AST que
	// llegó sin Level (un --filter externo, un JSON viejo). Los regexes de
	// abajo siguen siendo el camino real para ESE segundo caso — no un
	// fallback deprecado a borrar: si se eliminan, todo heading de un AST
	// sin Level se degrada a párrafo en silencio.
	if elem.Level > 0 {
		text := content
		if m := headingHTMLPattern.FindStringSubmatch(content); m != nil {
			text = m[1]
		} else if m := markdownHeadingPattern.FindStringSubmatch(content); m != nil {
			text = m[1]
		}
		return g.renderHeading(doc, text, elem.Level)
	}

	// Detectar headings HTML (del FlexParser, para un AST sin Level)
	if h2HTMLMatch := regexp.MustCompile(`^<h2[^>]*>(.+?)</h2>`).FindStringSubmatch(content); h2HTMLMatch != nil {
		return g.renderHeading(doc, h2HTMLMatch[1], 2)
	}
	if h3HTMLMatch := regexp.MustCompile(`^<h3[^>]*>(.+?)</h3>`).FindStringSubmatch(content); h3HTMLMatch != nil {
		return g.renderHeading(doc, h3HTMLMatch[1], 3)
	}
	if h4HTMLMatch := regexp.MustCompile(`^<h4[^>]*>(.+?)</h4>`).FindStringSubmatch(content); h4HTMLMatch != nil {
		return g.renderHeading(doc, h4HTMLMatch[1], 4)
	}

	// Detectar headings Markdown (##)
	if h2Match := regexp.MustCompile(`^## (.+)`).FindStringSubmatch(content); h2Match != nil {
		return g.renderHeading(doc, h2Match[1], 2)
	}
	if h3Match := regexp.MustCompile(`^### (.+)`).FindStringSubmatch(content); h3Match != nil {
		return g.renderHeading(doc, h3Match[1], 3)
	}
	if h4Match := regexp.MustCompile(`^#### (.+)`).FindStringSubmatch(content); h4Match != nil {
		return g.renderHeading(doc, h4Match[1], 4)
	}

	// Párrafo normal
	return g.renderParagraph(doc, content)
}

// renderHeading renderiza un encabezado H2/H3/H4
func (g *DOCXGenerator) renderHeading(doc domain.Document, text string, level int) error {
	p, err := doc.AddParagraph()
	if err != nil {
		return err
	}

	var styleID string
	var size, color string
	var bold bool
	var spaceBefore, spaceAfter string

	switch level {
	case 1:
		// H1 nunca llegaba acá vía los regexes de arriba (que solo
		// intentaban h2/h3/h4) aunque docx_styles.go define H1Size/H1Color/
		// etc. en los 4 temas — datos poblados y nunca usados. Level (issue
		// #22) sí puede traer un 1 real (un TextElement con un único `#`
		// fuera del título de sección), así que ahora se usa.
		styleID = string(domain.StyleIDHeading1)
		size = g.style.H1Size
		color = g.style.H1Color
		bold = g.style.H1Bold
		spaceBefore = g.style.H1SpaceBefore
		spaceAfter = g.style.H1SpaceAfter
	case 2:
		styleID = string(domain.StyleIDHeading2)
		size = g.style.H2Size
		color = g.style.H2Color
		bold = g.style.H2Bold
		spaceBefore = g.style.H2SpaceBefore
		spaceAfter = g.style.H2SpaceAfter
	case 3:
		styleID = string(domain.StyleIDHeading3)
		size = g.style.H3Size
		color = g.style.H3Color
		bold = g.style.H3Bold
		spaceBefore = g.style.H3SpaceBefore
		spaceAfter = g.style.H3SpaceAfter
	case 4:
		styleID = string(domain.StyleIDHeading4)
		size = g.style.H4Size
		color = g.style.H4Color
		bold = g.style.H4Bold
		spaceBefore = g.style.H4SpaceBefore
		spaceAfter = g.style.H4SpaceAfter
	default:
		// level 5/6 (Level tope en 6, ver document_flex.go's parseSubsectionHeader):
		// docx_styles.go no define un H5/H6 propio en ningún tema. Antes de
		// wirear Level, un <h5>/<h6>/`#####` nunca matcheaba ninguno de los
		// regexes (solo existían para h2/h3/h4) y siempre caía a párrafo
		// normal, perdiendo la semántica de heading por completo. Degradar
		// al estilo de H4 (el más profundo definido) preserva al menos esa
		// semántica en vez de un styleID vacío con tamaño/color en cero.
		styleID = string(domain.StyleIDHeading4)
		size = g.style.H4Size
		color = g.style.H4Color
		bold = g.style.H4Bold
		spaceBefore = g.style.H4SpaceBefore
		spaceAfter = g.style.H4SpaceAfter
	}

	_ = p.SetStyle(styleID)
	if err := p.SetSpacingBefore(g.parseTwips(spaceBefore)); err != nil {
		return fmt.Errorf("invalid spacing before: %w", err)
	}
	if err := p.SetSpacingAfter(g.parseTwips(spaceAfter)); err != nil {
		return fmt.Errorf("invalid spacing after: %w", err)
	}

	if err := g.renderHeadingInline(p, text, size, color, bold); err != nil {
		return err
	}

	// TODO: Agregar bookmark

	return nil
}

// docxHeadingTagPattern matches every open/close tag core's inline pipeline
// can emit for a single line — <strong>, <em>, <code>, <span lang="xx">, and
// (as a catch-all so this parses as MARKUP rather than falling through to
// literal text) any other tag the pipeline's other single-line passes can
// also produce for a heading (<mark>, <del>, <span class="...">,
// <a href="...">). Issue #63 code review finding #1: text is already-
// rendered HTML by the time a heading reaches the DOCX generator
// (parser.parseSubsectionHeader runs the full inline pipeline at parse
// time), so renderInlineMarkdown (which expects unrendered markdown SOURCE)
// cannot be reused here.
var docxHeadingTagPattern = regexp.MustCompile(`<(/?)(strong|em|code|mark|del|a|span)([^>]*)>`)

// docxHeadingSpanLangAttr extracts lang="xx" from a <span ...> open tag's
// attribute string.
var docxHeadingSpanLangAttr = regexp.MustCompile(`\blang="([^"]*)"`)

// renderHeadingInline parses html into one or more domain.Run, applying
// size/color/baseBold to every run plus bold/italic/code/language for
// whichever of the recognized tags (see docxHeadingTagPattern) each run of
// text is nested inside. mark/del/a and a <span> with no lang attribute
// (e.g. a class-styled span) are recognized as markup — so their contents
// don't show up as literal "<tag>" text (finding #1's original bug) — but
// carry no DOCX formatting of their own: out of scope for this fix, their
// text renders with whatever formatting is already active around them.
//
// Assumes well-nested tags, same posture as the rest of this codebase
// today: core's own inline pipeline has a separate, tracked cross-nesting
// defect (issue #63 code review findings #7/#8, fixed in its own PR) that
// this function does not attempt to work around.
//
// html with no recognized tags at all (e.g. raw Markdown source reaching
// here via the Level-less/markdownHeadingPattern fallback in renderText,
// an AST from an external filter) degrades to exactly one run with the
// literal string — identical to this function's pre-#1-fix behavior, so
// routing that path through here too is not a regression.
func (g *DOCXGenerator) renderHeadingInline(p domain.Paragraph, html string, size, color string, baseBold bool) error {
	type activeState struct {
		bold, italic, code bool
		lang               string
	}
	stack := []activeState{{bold: baseBold}}

	emit := func(text string) error {
		if text == "" {
			return nil
		}
		top := stack[len(stack)-1]
		r, err := p.AddRun()
		if err != nil {
			return err
		}
		_ = r.SetText(renderer.UnescapeHTML(text))
		if err := r.SetSize(g.parseSize(size)); err != nil {
			return fmt.Errorf("invalid font size: %w", err)
		}
		_ = r.SetColor(g.parseColor(color))
		_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
		if top.bold {
			_ = r.SetBold(true)
		}
		if top.italic {
			_ = r.SetItalic(true)
		}
		if top.code {
			_ = r.SetFont(domain.Font{Name: g.style.CodeFontFamily})
		}
		if top.lang != "" && a11y.IsValidLangTag(top.lang) {
			if err := r.SetLanguage(&domain.Language{Val: top.lang}); err != nil {
				return err
			}
		}
		return nil
	}

	pos := 0
	for pos < len(html) {
		loc := docxHeadingTagPattern.FindStringSubmatchIndex(html[pos:])
		if loc == nil {
			if err := emit(html[pos:]); err != nil {
				return err
			}
			break
		}
		tagStart, tagEnd := pos+loc[0], pos+loc[1]
		if tagStart > pos {
			if err := emit(html[pos:tagStart]); err != nil {
				return err
			}
		}
		closing := html[pos+loc[2]:pos+loc[3]] == "/"
		tagName := html[pos+loc[4] : pos+loc[5]]
		attrs := html[pos+loc[6] : pos+loc[7]]

		if closing {
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		} else {
			next := stack[len(stack)-1] // hereda el estado activo
			switch tagName {
			case "strong":
				next.bold = true
			case "em":
				next.italic = true
			case "code":
				next.code = true
			case "span":
				if m := docxHeadingSpanLangAttr.FindStringSubmatch(attrs); m != nil {
					next.lang = m[1]
				}
			}
			stack = append(stack, next)
		}
		pos = tagEnd
	}

	return nil
}

// docxStripHeadingMarkup reduces a heading source string — which may still
// contain markup emitted by core's inline pipeline (<span lang="xx">,
// <strong>, <em>, <code>, ...) — to plain text, for the two uses that can't
// render markup: the static TOC entry (renderTOC only writes plain text
// runs, see entryRun.SetText below) and sanitizeBookmarkID's ASCII
// allowlist. Issue #63 code review finding #1: before this, a lang span in
// a heading corrupted the bookmark ID into "Hola_spanlangfrmondespan"
// instead of "Hola_monde" (bookmarkIDDisallowedChars strips the markup's
// punctuation but not the tag names/attribute values it leaves behind).
func docxStripHeadingMarkup(s string) string {
	s = docxHeadingTagPattern.ReplaceAllString(s, "")
	return renderer.UnescapeHTML(s)
}

// renderParagraph renderiza un párrafo normal con markdown inline
func (g *DOCXGenerator) renderParagraph(doc domain.Document, content string) error {
	p, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := p.SetSpacingAfter(g.parseTwips(g.style.TextSpaceAfter)); err != nil {
		return fmt.Errorf("invalid spacing after: %w", err)
	}

	return g.renderInlineMarkdown(p, content)
}

// renderInlineMarkdown procesa markdown inline (**bold**, *italic*, `code`, [links])
func (g *DOCXGenerator) renderInlineMarkdown(p domain.Paragraph, content string) error {
	return g.walkDocxInlinePatterns(p, content, g.docxInlinePatterns(), nil)
}

// docxInlinePattern es un par regex+apply del parser de markdown inline del
// DOCX generator. apply recibe el paragraph (no un run ya creado — issue
// #63 code review finding #2: el patrón de idioma puede necesitar producir
// MÁS de un run, uno por cada tramo de negrita/cursiva/código dentro del
// span, así que cada apply crea sus propios runs), el texto interno (grupo
// de captura 1), el segundo grupo de captura si el patrón lo tiene (hoy
// solo el de idioma; el resto lo ignora), el texto completo matcheado (con
// delimitadores — el patrón de idioma lo usa para degradar a texto literal
// cuando el tag no valida, finding #5), y postRun: un hook opcional que
// corre sobre cada run creado — el patrón de idioma lo usa para propagar
// SetLanguage a cada run que produzca su recursión sobre el texto interno.
type docxInlinePattern struct {
	regex *regexp.Regexp
	apply func(p domain.Paragraph, text string, extra string, matchedText string, postRun func(r domain.Run) error) error
}

// docxSimpleRunApply factoriza el patrón repetido de los 4 patterns "de un
// solo run" (code/bold/italic/link): crear un run, aplicarle style, y
// propagar postRun si esta llamada viene de una recursión (finding #2).
func docxSimpleRunApply(style func(r domain.Run) error) func(p domain.Paragraph, text string, extra string, matchedText string, postRun func(r domain.Run) error) error {
	return func(p domain.Paragraph, text string, _ string, _ string, postRun func(r domain.Run) error) error {
		r, err := p.AddRun()
		if err != nil {
			return err
		}
		_ = r.SetText(text)
		if err := style(r); err != nil {
			return err
		}
		if postRun != nil {
			return postRun(r)
		}
		return nil
	}
}

// docxCodePattern, docxBoldPattern, docxItalicPattern, docxLinkPattern,
// docxLangPattern construyen cada uno de los 5 patterns por separado (issue
// #63 code review advisor follow-up on finding #2): docxInlinePatterns() y
// docxLangInnerPatterns() antes compartían este set via un slice de índice
// posicional ([:3]) — un invariante de ORDEN implícito y no verificado por
// ningún test; reordenar o insertar un pattern en docxInlinePatterns()
// habría hecho que la recursión de docxLangInnerPatterns() empezara a
// matchear links o lang-dentro-de-lang en silencio. Nombrarlos hace que
// docxLangInnerPatterns() elija por identidad, no por posición.

func (g *DOCXGenerator) docxCodePattern() docxInlinePattern {
	return docxInlinePattern{
		// `code` - código inline
		regex: regexp.MustCompile("`([^`]+)`"),
		apply: docxSimpleRunApply(func(r domain.Run) error {
			if err := r.SetSize(g.parseSize(g.style.FontSizeCode)); err != nil {
				return err
			}
			_ = r.SetColor(g.parseColor(g.style.CodeInlineColor))
			_ = r.SetFont(domain.Font{Name: g.style.CodeFontFamily})
			return nil
		}),
	}
}

func (g *DOCXGenerator) docxBoldPattern() docxInlinePattern {
	return docxInlinePattern{
		// **bold** - negrita
		regex: regexp.MustCompile(`\*\*([^*]+)\*\*`),
		apply: docxSimpleRunApply(func(r domain.Run) error {
			if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
				return err
			}
			_ = r.SetColor(g.parseColor(g.style.TextColor))
			_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
			_ = r.SetBold(true)
			return nil
		}),
	}
}

func (g *DOCXGenerator) docxItalicPattern() docxInlinePattern {
	return docxInlinePattern{
		// *italic* - cursiva
		regex: regexp.MustCompile(`\*([^*]+)\*`),
		apply: docxSimpleRunApply(func(r domain.Run) error {
			if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
				return err
			}
			_ = r.SetColor(g.parseColor(g.style.TextColor))
			_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
			_ = r.SetItalic(true)
			return nil
		}),
	}
}

func (g *DOCXGenerator) docxLinkPattern() docxInlinePattern {
	return docxInlinePattern{
		// [text](url) - links
		regex: regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`),
		apply: docxSimpleRunApply(func(r domain.Run) error {
			if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
				return err
			}
			_ = r.SetColor(g.parseColor(g.style.LinkColor))
			_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
			// Links con subrayado (usar UnderlineNone + 1 = single)
			_ = r.SetUnderline(domain.UnderlineStyle(1))
			// TODO: Agregar hyperlink real cuando docxgo lo soporte
			return nil
		}),
	}
}

func (g *DOCXGenerator) docxLangPattern() docxInlinePattern {
	return docxInlinePattern{
		// [text]{lang=xx} - idioma inline (issue #63). Regex compartido con
		// core y pptx.go (renderer.InlineLangSpanPattern) en vez de una
		// copia textual propia (hallazgo de regex triplicado del review).
		regex: renderer.InlineLangSpanPattern,
		apply: g.docxApplyLangSpan,
	}
}

// docxInlinePatterns es el set completo usado en prosa de cuerpo: code,
// bold, italic, links, idioma (issue #63) — en ese orden (code primero para
// evitar procesar ** dentro de `).
func (g *DOCXGenerator) docxInlinePatterns() []docxInlinePattern {
	return []docxInlinePattern{
		g.docxCodePattern(),
		g.docxBoldPattern(),
		g.docxItalicPattern(),
		g.docxLinkPattern(),
		g.docxLangPattern(),
	}
}

// docxLangInnerPatterns es el subconjunto usado para procesar el texto
// INTERNO de un [texto]{lang=xx} ya validado (finding #2): code/bold/italic
// solamente, nunca links ni otro span de idioma anidado — el content-class
// de InlineLangSpanPattern excluye "[", así que ninguno de los dos puede
// aparecer ahí adentro para empezar (mismo razonamiento que
// core/renderer/sanitizer.go's inlineSpanPattern doc comment sobre por qué
// excluir "[" evita el straddle).
func (g *DOCXGenerator) docxLangInnerPatterns() []docxInlinePattern {
	return []docxInlinePattern{g.docxCodePattern(), g.docxBoldPattern(), g.docxItalicPattern()}
}

// docxApplyLangSpan es el apply del patrón de idioma. Un tag inválido
// degrada a texto LITERAL (matchedText completo, con corchetes/llaves —
// finding #5) en vez de quedarse silenciosamente solo con el texto interno,
// que escondería el error de tipeo del autor en el documento final. Un tag
// válido procesa el texto interno recursivamente por code/bold/italic
// (finding #2) en vez de emitirlo verbatim con los asteriscos/backticks
// literales, y estampa el idioma en cada run que esa recursión produzca.
func (g *DOCXGenerator) docxApplyLangSpan(p domain.Paragraph, text string, lang string, matchedText string, postRun func(r domain.Run) error) error {
	if !a11y.IsValidLangTag(lang) {
		r, err := p.AddRun()
		if err != nil {
			return err
		}
		_ = r.SetText(matchedText)
		if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
			return err
		}
		_ = r.SetColor(g.parseColor(g.style.TextColor))
		_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
		if postRun != nil {
			return postRun(r)
		}
		return nil
	}
	setLang := func(r domain.Run) error {
		if err := r.SetLanguage(&domain.Language{Val: lang}); err != nil {
			return err
		}
		if postRun != nil {
			return postRun(r)
		}
		return nil
	}
	return g.walkDocxInlinePatterns(p, text, g.docxLangInnerPatterns(), setLang)
}

// walkDocxInlinePatterns es el parser de segmento-por-segmento compartido
// por renderInlineMarkdown (todo el set de patterns, postRun nil) y
// docxApplyLangSpan (subset restringido sobre el texto interno de un span
// de idioma ya validado, postRun = estampar el idioma). postRun corre sobre
// cada run que este walk crea directamente para el texto "de relleno" entre
// matches; los runs que crea un pattern.apply anidado (p.ej. bold dentro de
// un span de idioma) lo reciben porque cada apply lo recibe como argumento
// y decide propagarlo, no porque este walk los toque directamente.
func (g *DOCXGenerator) walkDocxInlinePatterns(p domain.Paragraph, content string, patterns []docxInlinePattern, postRun func(r domain.Run) error) error {
	remaining := content
	pos := 0

	for pos < len(remaining) {
		// Buscar el próximo match de cualquier pattern
		minPos := len(remaining)
		var matchedPattern *docxInlinePattern
		var matchedText string
		var matchedInner string
		var matchedExtra string

		for i := range patterns {
			pattern := &patterns[i]
			loc := pattern.regex.FindStringSubmatchIndex(remaining[pos:])
			if loc != nil && loc[0] < minPos {
				minPos = loc[0]
				matchedPattern = pattern
				matchedText = remaining[pos+loc[0] : pos+loc[1]]
				matchedInner = ""
				matchedExtra = ""
				if len(loc) >= 4 {
					matchedInner = remaining[pos+loc[2] : pos+loc[3]]
				}
				if len(loc) >= 6 {
					matchedExtra = remaining[pos+loc[4] : pos+loc[5]]
				}
			}
		}

		// Agregar texto antes del match
		if minPos > 0 {
			// Validar límites antes de slicing
			endPos := pos + minPos
			if endPos > len(remaining) {
				endPos = len(remaining)
			}
			r, err := p.AddRun()
			if err != nil {
				return err
			}
			_ = r.SetText(remaining[pos:endPos])
			if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
				return err
			}
			_ = r.SetColor(g.parseColor(g.style.TextColor))
			_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
			if postRun != nil {
				if err := postRun(r); err != nil {
					return err
				}
			}
			pos += minPos
		}

		// Agregar texto con formato
		if matchedPattern != nil {
			if err := matchedPattern.apply(p, matchedInner, matchedExtra, matchedText, postRun); err != nil {
				return err
			}
			pos += len(matchedText)
		} else {
			// No más matches, agregar el resto
			if pos < len(remaining) {
				r, err := p.AddRun()
				if err != nil {
					return err
				}
				_ = r.SetText(remaining[pos:])
				if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
					return err
				}
				_ = r.SetColor(g.parseColor(g.style.TextColor))
				_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
				if postRun != nil {
					if err := postRun(r); err != nil {
						return err
					}
				}
			}
			break
		}
	}

	return nil
}

// Stubs para elementos restantes (implementar después)

func (g *DOCXGenerator) renderPoints(doc domain.Document, elem *ast.PointsElement) error {
	for i, item := range elem.Items {
		p, err := doc.AddParagraph()
		if err != nil {
			return err
		}

		// Indentación para listas
		indent := g.parseTwips(g.style.ListIndent)
		if err := p.SetIndent(domain.Indentation{Left: indent}); err != nil {
			return fmt.Errorf("invalid indent: %w", err)
		}
		if err := p.SetSpacingAfter(g.parseTwips(g.style.TextSpaceAfter) / 2); err != nil {
			return fmt.Errorf("invalid spacing after: %w", err)
		}

		// Bullet o número
		r, err := p.AddRun()
		if err != nil {
			return err
		}

		bullet := "• "
		if elem.ListType == "ordered" {
			bullet = fmt.Sprintf("%d. ", i+1)
		}

		_ = r.SetText(bullet)
		if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
			return fmt.Errorf("invalid font size: %w", err)
		}
		_ = r.SetColor(g.parseColor(g.style.TextColor))
		_ = r.SetFont(domain.Font{Name: g.style.FontFamily})

		// Contenido del item con markdown inline
		if err := g.renderInlineMarkdown(p, item.Content); err != nil {
			return err
		}
	}

	return nil
}

func (g *DOCXGenerator) renderCode(doc domain.Document, elem *ast.CodeElement) error {
	// Párrafo para código con fondo y monospace
	lines := strings.Split(elem.Content, "\n")

	for _, line := range lines {
		p, err := doc.AddParagraph()
		if err != nil {
			return err
		}

		// Indentación ligera
		if err := p.SetIndent(domain.Indentation{Left: 360}); err != nil {
			return fmt.Errorf("invalid indent: %w", err)
		}
		if err := p.SetSpacingAfter(0); err != nil {
			return fmt.Errorf("invalid spacing after: %w", err)
		}

		// TODO: Agregar fondo (shading) cuando docxgo lo soporte

		r, err := p.AddRun()
		if err != nil {
			return err
		}

		// Monospace font
		_ = r.SetText(line)
		if err := r.SetSize(g.parseSize(g.style.FontSizeCode)); err != nil {
			return fmt.Errorf("invalid font size: %w", err)
		}
		_ = r.SetColor(g.parseColor(g.style.CodeBlockColor))
		_ = r.SetFont(domain.Font{Name: g.style.CodeFontFamily})
	}

	// Espacio después del bloque
	spacer, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := spacer.SetSpacingAfter(g.parseTwips(g.style.TextSpaceAfter)); err != nil {
		return fmt.Errorf("invalid spacing after: %w", err)
	}

	return nil
}

func (g *DOCXGenerator) renderTable(doc domain.Document, elem *ast.TableElement) error {
	// Crear tabla: headers + rows
	totalRows := 1 + len(elem.Rows) // headers + data rows
	if len(elem.Headers) == 0 {
		return nil
	}

	numCols := len(elem.Headers)
	table, err := doc.AddTable(totalRows, numCols)
	if err != nil {
		return err
	}

	// Header row
	headerRow, err := table.Row(0)
	if err != nil {
		return err
	}

	for j, header := range elem.Headers {
		cell, err := headerRow.Cell(j)
		if err != nil {
			continue
		}

		p, err := cell.AddParagraph()
		if err != nil {
			continue
		}

		r, err := p.AddRun()
		if err != nil {
			continue
		}
		_ = r.SetText(header)
		_ = r.SetBold(true)
		if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
			continue
		}
		_ = r.SetColor(g.parseColor(g.style.TableHeaderColor))
		_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
	}

	// Data rows
	for i, row := range elem.Rows {
		dataRow, err := table.Row(i + 1)
		if err != nil {
			continue
		}

		for j, cellContent := range row {
			if j >= numCols {
				break
			}

			cell, err := dataRow.Cell(j)
			if err != nil {
				continue
			}

			p, err := cell.AddParagraph()
			if err != nil {
				continue
			}

			if err := g.renderInlineMarkdown(p, cellContent); err != nil {
				return err
			}
		}
	}

	// Espacio después
	spacer, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := spacer.SetSpacingAfter(g.parseTwips(g.style.TextSpaceAfter)); err != nil {
		return fmt.Errorf("invalid spacing after: %w", err)
	}

	return nil
}

// renderImagePlaceholder inserta el texto itálico placeholder que reemplaza
// una imagen no insertada (no encontrada o bloqueada por confinamiento).
func (g *DOCXGenerator) renderImagePlaceholder(p domain.Paragraph, text string) error {
	r, err := p.AddRun()
	if err != nil {
		return err
	}
	_ = r.SetText(text)
	if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
		return fmt.Errorf("invalid font size: %w", err)
	}
	_ = r.SetColor(g.parseColor(g.style.TextLightColor))
	_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
	_ = r.SetItalic(true)
	return nil
}

func (g *DOCXGenerator) renderImage(doc domain.Document, elem *ast.ImageElement) error {
	// Agregar párrafo para la imagen
	p, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := p.SetAlignment(domain.AlignmentCenter); err != nil {
		return fmt.Errorf("invalid alignment: %w", err)
	}

	// Insertar imagen usando docxgo v2. elem.Source es contenido del
	// documento (no confiable): docxgo lee el path directo con os.ReadFile
	// sin ninguna sanitización propia, a diferencia del path HTML (que sí
	// aplica SanitizeURL) — sin confinar aquí, una ruta absoluta o con ".."
	// copiaría un archivo local arbitrario a word/media/ del .docx generado
	// (ver docs/SECURITY_AUDIT_2026-07.md, AL-4).
	imagePath := elem.Source
	if g.assetRoot != "" {
		confined, err := util.ResolveConfinedPath(g.assetRoot, imagePath)
		if err != nil {
			g.logger.Warn("DOCX: Image source blocked (outside asset root): %s: %v", imagePath, err)
			return g.renderImagePlaceholder(p, fmt.Sprintf("[Image blocked: %s]", imagePath))
		}
		imagePath = confined
	}

	// Tamaño por defecto: 6 pulgadas de ancho (mantiene aspect ratio)
	imageSize := domain.NewImageSizeInches(6.0, 0) // 0 = mantener proporción

	// Agregar imagen
	img, err := p.AddImageWithSize(imagePath, imageSize)
	if err != nil {
		g.logger.Warn("DOCX: Failed to insert image %s: %v", imagePath, err)
		return g.renderImagePlaceholder(p, fmt.Sprintf("[Image not found: %s]", imagePath))
	}

	_ = img // Imagen insertada exitosamente
	g.logger.Info("DOCX", "✅ Image inserted: %s", imagePath)

	// Caption si existe
	if elem.Caption != "" {
		captionPara, err := doc.AddParagraph()
		if err != nil {
			return err
		}
		if err := captionPara.SetAlignment(domain.AlignmentCenter); err != nil {
			return fmt.Errorf("invalid alignment: %w", err)
		}
		if err := captionPara.SetSpacingAfter(g.parseTwips(g.style.TextSpaceAfter)); err != nil {
			return fmt.Errorf("invalid spacing after: %w", err)
		}

		r, err := captionPara.AddRun()
		if err != nil {
			return err
		}
		// issue #239: Number lo asigna xref.Transform (built-in de #240)
		// antes de renderizar; sin Label nunca corrió y Number es 0.
		captionText := elem.Caption
		if elem.Label != "" && elem.Number > 0 {
			captionText = fmt.Sprintf("Figura %d: %s", elem.Number, elem.Caption)
		}
		_ = r.SetText(captionText)
		if err := r.SetSize(g.parseSize(g.style.FontSizeBase) - 2); err != nil {
			return fmt.Errorf("invalid font size: %w", err)
		}
		_ = r.SetColor(g.parseColor(g.style.TextLightColor))
		_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
		_ = r.SetItalic(true)
	}

	return nil
}

func (g *DOCXGenerator) renderMermaid(doc domain.Document, elem *ast.MermaidElement) error {
	if g.chromiumRenderer == nil {
		g.logger.Warn("DOCX: Chromium not available, skipping mermaid diagram")
		return nil
	}

	g.logger.Info("DOCX", "Rendering Mermaid diagram (%s)...", elem.DiagramType)

	// Renderizar a PNG usando ChromiumRenderer con mayor resolución
	// Usar dimensiones más grandes para que Mermaid tenga más espacio
	pngBytes, err := g.chromiumRenderer.RenderMermaidToPNG(context.Background(), elem.Content, 2400, 1600)
	if err != nil {
		g.logger.Warn("DOCX: Failed to render mermaid: %v", err)
		return g.renderPlaceholder(doc, fmt.Sprintf("Mermaid Diagram: %s (render failed)", elem.DiagramType))
	}

	// Guardar PNG temporalmente
	pngPath := fmt.Sprintf("%s/mermaid_%s.png", g.tempDir, sanitizeBookmarkID(elem.DiagramType))
	if err := os.WriteFile(pngPath, pngBytes, 0644); err != nil {
		return err
	}

	// Insertar imagen en documento
	p, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := p.SetAlignment(domain.AlignmentCenter); err != nil {
		return fmt.Errorf("invalid alignment: %w", err)
	}

	imageSize := domain.NewImageSizeInches(6.5, 0) // 6.5" ancho, altura proporcional
	img, err := p.AddImageWithSize(pngPath, imageSize)
	if err != nil {
		g.logger.Warn("DOCX: Failed to insert mermaid image: %v", err)
		return g.renderPlaceholder(doc, fmt.Sprintf("Mermaid Diagram: %s", elem.DiagramType))
	}

	if img == nil {
		g.logger.Warn("DOCX: ⚠️  Mermaid image object is nil after insertion")
	}

	sizeKB := float64(len(pngBytes)) / 1024
	g.logger.Info("DOCX", "✅ Mermaid inserted (%.1f KB)", sizeKB)

	return nil
}

// renderMath rasteriza una ecuación LaTeX a PNG e la inserta (issue #239-B)
// — mismo patrón que renderMermaid: DOCX no puede embeber SVG/MathML, así
// que MathJax→SVG→PNG vía Chromium es la ruta pragmática. Fidelidad
// LaTeX→OMML nativa queda fuera de alcance (limitación documentada, misma
// clase que las ya conocidas de docx.go: bookmarks/hyperlinks reales).
func (g *DOCXGenerator) renderMath(doc domain.Document, elem *ast.MathElement) error {
	if g.chromiumRenderer == nil {
		g.logger.Warn("DOCX: Chromium not available, skipping equation")
		return nil
	}

	g.logger.Info("DOCX", "Rendering equation...")

	pngBytes, err := g.chromiumRenderer.RenderMathToPNG(context.Background(), elem.Content, 1600, 400)
	if err != nil {
		g.logger.Warn("DOCX: Failed to render equation: %v", err)
		return g.renderPlaceholder(doc, "Equation (render failed)")
	}

	pngPath := fmt.Sprintf("%s/math_%s.png", g.tempDir, chromium.GenerateContentHash(elem.Content)[:12])
	if err := os.WriteFile(pngPath, pngBytes, 0644); err != nil {
		return err
	}

	p, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := p.SetAlignment(domain.AlignmentCenter); err != nil {
		return fmt.Errorf("invalid alignment: %w", err)
	}

	imageSize := domain.NewImageSizeInches(4.0, 0)
	img, err := p.AddImageWithSize(pngPath, imageSize)
	if err != nil {
		g.logger.Warn("DOCX: Failed to insert equation image: %v", err)
		return g.renderPlaceholder(doc, "Equation")
	}
	if img == nil {
		g.logger.Warn("DOCX: ⚠️  Equation image object is nil after insertion")
	}

	// issue #239: Number lo asigna xref.Transform (built-in de #240) antes
	// de renderizar; sin Label nunca corrió y Number es 0. "(N)" es su
	// propia línea, separado del caption — mismo convenio que renderMathElement
	// (renderer/html.go).
	if elem.Label != "" && elem.Number > 0 {
		numPara, err := doc.AddParagraph()
		if err != nil {
			return err
		}
		if err := numPara.SetAlignment(domain.AlignmentCenter); err != nil {
			return fmt.Errorf("invalid alignment: %w", err)
		}
		r, err := numPara.AddRun()
		if err != nil {
			return err
		}
		_ = r.SetText(fmt.Sprintf("(%d)", elem.Number))
		_ = r.SetColor(g.parseColor(g.style.TextLightColor))
		_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
	}

	if elem.Caption != "" {
		captionPara, err := doc.AddParagraph()
		if err != nil {
			return err
		}
		if err := captionPara.SetAlignment(domain.AlignmentCenter); err != nil {
			return fmt.Errorf("invalid alignment: %w", err)
		}
		r, err := captionPara.AddRun()
		if err != nil {
			return err
		}
		_ = r.SetText(elem.Caption)
		if err := r.SetSize(g.parseSize(g.style.FontSizeBase) - 2); err != nil {
			return fmt.Errorf("invalid font size: %w", err)
		}
		_ = r.SetColor(g.parseColor(g.style.TextLightColor))
		_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
		_ = r.SetItalic(true)
	}

	sizeKB := float64(len(pngBytes)) / 1024
	g.logger.Info("DOCX", "✅ Equation inserted (%.1f KB)", sizeKB)

	return nil
}

func (g *DOCXGenerator) renderChart(doc domain.Document, elem *ast.ChartElement) error {
	if g.chromiumRenderer == nil {
		g.logger.Warn("DOCX: Chromium not available, skipping chart")
		return nil
	}

	g.logger.Info("DOCX", "Rendering Chart.js chart (%s)...", elem.ChartType)

	// Generar configuración de Chart.js optimizada para exportación a PNG
	chartConfig := renderer.GenerateChartConfigForExport(elem)

	// Renderizar a PNG usando ChromiumRenderer con alta resolución para mejor calidad en Word
	// 2400x1500 pixels = buena calidad para impresión y pantalla
	pngBytes, err := g.chromiumRenderer.RenderChartToPNG(context.Background(), chartConfig, 2400, 1500)
	if err != nil {
		g.logger.Warn("DOCX: Failed to render chart: %v", err)
		return g.renderPlaceholder(doc, fmt.Sprintf("Chart: %s (render failed)", elem.ChartType))
	}

	// Guardar PNG temporalmente
	pngPath := fmt.Sprintf("%s/chart_%s.png", g.tempDir, sanitizeBookmarkID(elem.ChartType))
	if err := os.WriteFile(pngPath, pngBytes, 0644); err != nil {
		return err
	}

	// Insertar imagen en documento
	p, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := p.SetAlignment(domain.AlignmentCenter); err != nil {
		return fmt.Errorf("invalid alignment: %w", err)
	}

	imageSize := domain.NewImageSizeInches(6.5, 4.1) // 6.5" ancho x 4.1" alto (ratio 1.6:1)
	img, err := p.AddImageWithSize(pngPath, imageSize)
	if err != nil {
		g.logger.Warn("DOCX: Failed to insert chart image: %v", err)
		return g.renderPlaceholder(doc, fmt.Sprintf("Chart: %s", elem.ChartType))
	}

	if img == nil {
		g.logger.Warn("DOCX: ⚠️  Image object is nil after insertion")
	} else {
		g.logger.Debug("DOCX", "Image object created successfully")
	}

	sizeKB := float64(len(pngBytes)) / 1024
	g.logger.Info("DOCX", "✅ Chart inserted (%.1f KB)", sizeKB)

	return nil
}

func (g *DOCXGenerator) renderMap(doc domain.Document, elem *ast.MapElement) error {
	if g.chromiumRenderer == nil {
		g.logger.Warn("DOCX: Chromium not available, skipping map")
		return nil
	}

	g.logger.Info("DOCX", "Rendering Leaflet map (%s, zoom=%d)...", elem.MapType, elem.Zoom)

	// Convertir ast.MapElement a renderer.MapConfig
	mapConfig := renderer.MapConfig{
		Zoom:    elem.Zoom,
		MapType: elem.MapType,
		Heatmap: elem.Heatmap,
	}

	// Set center if provided, otherwise use default (0, 0)
	if elem.Center != nil {
		mapConfig.CenterLat = elem.Center.Lat
		mapConfig.CenterLng = elem.Center.Lng
	}

	// Convertir markers
	for _, m := range elem.Markers {
		mapConfig.Markers = append(mapConfig.Markers, renderer.MapMarker{
			Lat:   m.Lat,
			Lng:   m.Lng,
			Label: m.Label,
			Color: m.Color,
		})
	}

	// Renderizar a PNG
	pngBytes, err := g.chromiumRenderer.RenderMapToPNG(context.Background(), mapConfig, 1200, 800)
	if err != nil {
		g.logger.Warn("DOCX: Failed to render map: %v", err)
		return g.renderPlaceholder(doc, fmt.Sprintf("Map: %s (render failed)", elem.MapType))
	}

	// Guardar PNG temporalmente
	pngPath := fmt.Sprintf("%s/map_%s.png", g.tempDir, sanitizeBookmarkID(elem.MapType))
	if err := os.WriteFile(pngPath, pngBytes, 0644); err != nil {
		return err
	}

	// Insertar imagen en documento
	p, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := p.SetAlignment(domain.AlignmentCenter); err != nil {
		return fmt.Errorf("invalid alignment: %w", err)
	}

	imageSize := domain.NewImageSizeInches(6.0, 0)
	img, err := p.AddImageWithSize(pngPath, imageSize)
	if err != nil {
		g.logger.Warn("DOCX: Failed to insert map image: %v", err)
		return g.renderPlaceholder(doc, fmt.Sprintf("Map: %s", elem.MapType))
	}

	if img == nil {
		g.logger.Warn("DOCX: ⚠️  Map image object is nil after insertion")
	}

	sizeKB := float64(len(pngBytes)) / 1024
	g.logger.Info("DOCX", "✅ Map inserted (%.1f KB)", sizeKB)

	return nil
}

func (g *DOCXGenerator) renderQuote(doc domain.Document, elem *ast.QuoteElement) error {
	p, err := doc.AddParagraph()
	if err != nil {
		return err
	}

	// Indentación para quotes
	if err := p.SetIndent(domain.Indentation{Left: 720}); err != nil {
		return fmt.Errorf("invalid indent: %w", err)
	}
	if err := p.SetSpacingAfter(g.parseTwips(g.style.TextSpaceAfter)); err != nil {
		return fmt.Errorf("invalid spacing after: %w", err)
	}

	// Contenido con markdown inline
	return g.renderInlineMarkdown(p, elem.Content)
}

func (g *DOCXGenerator) renderChecklist(doc domain.Document, elem *ast.ChecklistElement) error {
	for _, item := range elem.Items {
		p, err := doc.AddParagraph()
		if err != nil {
			return err
		}

		if err := p.SetIndent(domain.Indentation{Left: 360}); err != nil {
			return fmt.Errorf("invalid indent: %w", err)
		}
		if err := p.SetSpacingAfter(g.parseTwips(g.style.TextSpaceAfter) / 2); err != nil {
			return fmt.Errorf("invalid spacing after: %w", err)
		}

		// Checkbox symbol
		r, err := p.AddRun()
		if err != nil {
			return err
		}

		checkbox := "☐ "
		if item.Checked {
			checkbox = "☑ "
		}

		_ = r.SetText(checkbox)
		if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
			return fmt.Errorf("invalid font size: %w", err)
		}
		_ = r.SetColor(g.parseColor(g.style.TextColor))
		_ = r.SetFont(domain.Font{Name: g.style.FontFamily})

		// Contenido
		if err := g.renderInlineMarkdown(p, item.Content); err != nil {
			return err
		}
	}

	return nil
}

// renderSpecialBlock renderiza bloques especiales (info, warning, danger, success, tip)
func (g *DOCXGenerator) renderSpecialBlock(doc domain.Document, elem *ast.SpecialBlockElement) error {
	// Configuración de colores y emojis por tipo de bloque
	blockConfig := map[string]struct {
		emoji string
		color domain.Color
		bg    domain.Color
	}{
		"info":    {emoji: "ℹ️", color: domain.Color{R: 31, G: 119, B: 180}, bg: domain.Color{R: 230, G: 244, B: 255}}, // Azul
		"warning": {emoji: "⚠️", color: domain.Color{R: 255, G: 152, B: 0}, bg: domain.Color{R: 255, G: 243, B: 224}},  // Naranja
		"danger":  {emoji: "🚨", color: domain.Color{R: 244, G: 67, B: 54}, bg: domain.Color{R: 255, G: 235, B: 238}},   // Rojo
		"success": {emoji: "✅", color: domain.Color{R: 76, G: 175, B: 80}, bg: domain.Color{R: 232, G: 245, B: 233}},   // Verde
		"tip":     {emoji: "💡", color: domain.Color{R: 156, G: 39, B: 176}, bg: domain.Color{R: 243, G: 229, B: 245}},  // Púrpura
	}

	config, ok := blockConfig[elem.BlockType]
	if !ok {
		// Tipo desconocido, usar info por defecto
		config = blockConfig["info"]
	}

	// Emoji + Título (si existe)
	titleText := config.emoji
	if elem.Title != "" {
		titleText += " " + elem.Title
	} else {
		// Título por defecto basado en el tipo
		defaultTitles := map[string]string{
			"info":    "Información",
			"warning": "Advertencia",
			"danger":  "Peligro",
			"success": "Éxito",
			"tip":     "Consejo",
		}
		if defaultTitle, exists := defaultTitles[elem.BlockType]; exists {
			titleText += " " + defaultTitle
		}
	}

	// Párrafo de título
	titlePara, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := titlePara.SetSpacingBefore(240); err != nil {
		return fmt.Errorf("invalid spacing before: %w", err)
	}
	if err := titlePara.SetSpacingAfter(60); err != nil {
		return fmt.Errorf("invalid spacing after: %w", err)
	}
	if err := titlePara.SetIndent(domain.Indentation{Left: 360}); err != nil {
		return fmt.Errorf("invalid indent: %w", err)
	}

	titleRun, err := titlePara.AddRun()
	if err != nil {
		return err
	}
	_ = titleRun.SetText(titleText)
	if err := titleRun.SetSize(g.parseSize(g.style.FontSizeBase) + 2); err != nil {
		return fmt.Errorf("invalid font size: %w", err)
	}
	_ = titleRun.SetColor(config.color)
	_ = titleRun.SetFont(domain.Font{Name: g.style.FontFamily})
	_ = titleRun.SetBold(true)

	// Contenido del bloque
	contentLines := strings.Split(elem.Content, "\n")
	for _, line := range contentLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		contentPara, err := doc.AddParagraph()
		if err != nil {
			return err
		}
		if err := contentPara.SetIndent(domain.Indentation{Left: 720}); err != nil {
			return fmt.Errorf("invalid indent: %w", err)
		}
		if err := contentPara.SetSpacingAfter(g.parseTwips(g.style.TextSpaceAfter) / 2); err != nil {
			return fmt.Errorf("invalid spacing after: %w", err)
		}

		// Renderizar con soporte de inline markdown
		if err := g.renderInlineMarkdown(contentPara, line); err != nil {
			return err
		}
	}

	// Espacio después del bloque
	spacer, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := spacer.SetSpacingAfter(120); err != nil {
		return fmt.Errorf("invalid spacing after: %w", err)
	}

	return nil
}

// renderCodeGroup renderiza un grupo de código con múltiples pestañas/lenguajes
func (g *DOCXGenerator) renderCodeGroup(doc domain.Document, elem *ast.CodeGroupElement) error {
	// Renderizar cada bloque de código con su etiqueta
	for i, block := range elem.CodeBlocks {
		// Etiqueta del lenguaje/pestaña
		labelPara, err := doc.AddParagraph()
		if err != nil {
			return err
		}
		if err := labelPara.SetSpacingBefore(120); err != nil {
			return fmt.Errorf("invalid spacing before: %w", err)
		}
		if err := labelPara.SetSpacingAfter(30); err != nil {
			return fmt.Errorf("invalid spacing after: %w", err)
		}

		labelRun, err := labelPara.AddRun()
		if err != nil {
			return err
		}

		label := block.Label
		if label == "" {
			label = block.Language
		}
		if label == "" {
			label = fmt.Sprintf("Code %d", i+1)
		}

		_ = labelRun.SetText(fmt.Sprintf("▸ %s", label))
		if err := labelRun.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
			return fmt.Errorf("invalid font size: %w", err)
		}
		_ = labelRun.SetColor(domain.Color{R: 100, G: 100, B: 100})
		_ = labelRun.SetFont(domain.Font{Name: g.style.FontFamily})
		_ = labelRun.SetBold(true)

		// Bloque de código
		codePara, err := doc.AddParagraph()
		if err != nil {
			return err
		}
		if err := codePara.SetIndent(domain.Indentation{Left: 360}); err != nil {
			return fmt.Errorf("invalid indent: %w", err)
		}
		if err := codePara.SetSpacingAfter(120); err != nil {
			return fmt.Errorf("invalid spacing after: %w", err)
		}

		codeRun, err := codePara.AddRun()
		if err != nil {
			return err
		}
		_ = codeRun.SetText(block.Content)
		if err := codeRun.SetSize(g.parseSize(g.style.FontSizeCode)); err != nil {
			return fmt.Errorf("invalid font size: %w", err)
		}
		_ = codeRun.SetColor(g.parseColor(g.style.CodeBlockColor))
		_ = codeRun.SetFont(domain.Font{Name: g.style.CodeFontFamily})
	}

	return nil
}

// renderPlantUML renderiza diagramas PlantUML descargando PNG del servidor
func (g *DOCXGenerator) renderPlantUML(doc domain.Document, elem *ast.PlantUMLElement) error {
	g.logger.Info("DOCX", "Rendering PlantUML diagram (%s)...", elem.DiagramType)

	// Crear PlantUMLFetcher para descargar la imagen
	fetcher := chromium.NewPlantUMLFetcher(
		"https://www.plantuml.com/plantuml",
		"png",
		g.tempDir,
	)

	// Descargar diagrama a archivo PNG
	assetPath, err := fetcher.FetchDiagramToAssets(context.Background(), elem.Content)
	if err != nil {
		g.logger.Warn("DOCX: Failed to fetch PlantUML diagram: %v", err)
		return g.renderPlaceholder(doc, fmt.Sprintf("PlantUML diagram failed: %s", elem.DiagramType))
	}

	// Construir path completo
	pngPath := fmt.Sprintf("%s/%s", g.tempDir, assetPath)

	// Insertar imagen
	p, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := p.SetAlignment(domain.AlignmentCenter); err != nil {
		return fmt.Errorf("invalid alignment: %w", err)
	}

	imageSize := domain.NewImageSizeInches(6.0, 0)
	_, err = p.AddImageWithSize(pngPath, imageSize)
	if err != nil {
		g.logger.Warn("DOCX: Failed to insert PlantUML image: %v", err)
		return g.renderPlaceholder(doc, "PlantUML image insertion failed")
	}

	// Obtener tamaño del archivo
	fileInfo, _ := os.Stat(pngPath)
	sizeKB := 0.0
	if fileInfo != nil {
		sizeKB = float64(fileInfo.Size()) / 1024.0
	}

	g.logger.Info("DOCX", "✅ PlantUML inserted (%.1f KB)", sizeKB)

	// Título si existe
	if elem.Title != "" {
		captionPara, err := doc.AddParagraph()
		if err != nil {
			return err
		}
		if err := captionPara.SetAlignment(domain.AlignmentCenter); err != nil {
			return fmt.Errorf("invalid alignment: %w", err)
		}
		if err := captionPara.SetSpacingAfter(g.parseTwips(g.style.TextSpaceAfter)); err != nil {
			return fmt.Errorf("invalid spacing after: %w", err)
		}

		captionRun, err := captionPara.AddRun()
		if err != nil {
			return err
		}
		_ = captionRun.SetText(elem.Title)
		if err := captionRun.SetSize(g.parseSize(g.style.FontSizeBase) - 2); err != nil {
			return fmt.Errorf("invalid font size: %w", err)
		}
		_ = captionRun.SetColor(g.parseColor(g.style.TextLightColor))
		_ = captionRun.SetFont(domain.Font{Name: g.style.FontFamily})
		_ = captionRun.SetItalic(true)
	}

	return nil
}

// renderGrid renderiza un layout de grid con columnas
func (g *DOCXGenerator) renderGrid(doc domain.Document, elem *ast.GridElement) error {
	// En DOCX, no tenemos grids verdaderos, así que renderizamos cada columna
	// secuencialmente con un separador visual

	// Prosa suelta dentro del grid pero fuera de cualquier columna (issue #9)
	if elem.Content != "" {
		if err := g.renderParagraph(doc, elem.Content); err != nil {
			return err
		}
	}

	for i, column := range elem.Columns {
		// Título de columna (opcional, basado en el número)
		if len(elem.Columns) > 1 {
			headerPara, err := doc.AddParagraph()
			if err != nil {
				return err
			}
			if err := headerPara.SetSpacingBefore(120); err != nil {
				return fmt.Errorf("invalid spacing before: %w", err)
			}
			if err := headerPara.SetSpacingAfter(60); err != nil {
				return fmt.Errorf("invalid spacing after: %w", err)
			}

			headerRun, err := headerPara.AddRun()
			if err != nil {
				return err
			}
			_ = headerRun.SetText(fmt.Sprintf("• Columna %d", i+1))
			if err := headerRun.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
				return fmt.Errorf("invalid font size: %w", err)
			}
			_ = headerRun.SetColor(domain.Color{R: 120, G: 120, B: 120})
			_ = headerRun.SetFont(domain.Font{Name: g.style.FontFamily})
			_ = headerRun.SetItalic(true)
		}

		// Renderizar el contenido de la columna con indentación. parseColumn
		// (core/elements/grid.go) solo puebla column.Content, nunca
		// column.Elements — iterar Elements aquí siempre estaba vacío y las
		// columnas de un grid en DOCX renderizaban sin texto (issue #56).
		// Cada línea de Content se procesa como un TextElement independiente,
		// reusando renderText para que un "## "/"### " dentro de una columna
		// siga detectándose como heading.
		originalSpacing := g.style.TextSpaceAfter
		g.style.TextSpaceAfter = fmt.Sprintf("%dpt", g.parseSize(g.style.TextSpaceAfter)/2)

		for _, line := range strings.Split(column.Content, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			textElem := ast.NewTextElement(column.GetPosition(), line)
			if err := g.renderText(doc, textElem); err != nil {
				g.style.TextSpaceAfter = originalSpacing
				return err
			}
		}

		g.style.TextSpaceAfter = originalSpacing

		// Separador entre columnas (excepto la última)
		if i < len(elem.Columns)-1 {
			sepPara, err := doc.AddParagraph()
			if err != nil {
				return err
			}
			if err := sepPara.SetSpacingAfter(120); err != nil {
				return fmt.Errorf("invalid spacing after: %w", err)
			}

			sepRun, err := sepPara.AddRun()
			if err != nil {
				return err
			}
			_ = sepRun.SetText("─────────────────────")
			if err := sepRun.SetSize(g.parseSize(g.style.FontSizeBase) - 2); err != nil {
				return fmt.Errorf("invalid font size: %w", err)
			}
			_ = sepRun.SetColor(domain.Color{R: 200, G: 200, B: 200})
			_ = sepRun.SetFont(domain.Font{Name: g.style.FontFamily})
		}
	}

	// Espacio después del grid
	spacer, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := spacer.SetSpacingAfter(240); err != nil {
		return fmt.Errorf("invalid spacing after: %w", err)
	}

	return nil
}

// renderPlaceholder renderiza un placeholder de texto para elementos que fallaron
func (g *DOCXGenerator) renderPlaceholder(doc domain.Document, text string) error {
	p, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := p.SetAlignment(domain.AlignmentCenter); err != nil {
		return fmt.Errorf("invalid alignment: %w", err)
	}

	r, err := p.AddRun()
	if err != nil {
		return err
	}
	_ = r.SetText(fmt.Sprintf("[%s]", text))
	if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
		return fmt.Errorf("invalid font size: %w", err)
	}
	_ = r.SetColor(g.parseColor(g.style.TextLightColor))
	_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
	_ = r.SetItalic(true)

	return nil
}

// renderMedia renderiza un MediaElement vía AddHyperlink (issue #36) —
// docxgo no soporta embeber audio/video real (ni <p:pic> con videoFile ni
// p:media, ver github.com/mmonterroca/pptxgo para el mismo límite del lado
// PPTX). NOTA: en docxgo v2.1.1, AddHyperlink registra la relación
// (relManager.AddHyperlink) pero NO envuelve el run en un <w:hyperlink
// r:id="...">  — solo aplica el estilo visual (azul + subrayado) al texto
// (ver internal/core/paragraph.go:100-132 del módulo vendido). El resultado
// es texto con apariencia de link, NO un link clickeable real — un bug de
// la librería, no de este código. Se usa igual, en vez de armar el estilo a
// mano, porque (a) muestra la URL real como texto visible en vez de un
// placeholder genérico, que sigue siendo estrictamente mejor que perder el
// dato, y (b) si docxgo corrige AddHyperlink en una versión futura, este
// código empieza a producir un link real sin cambios. Mismas reglas de
// seguridad que el path HTML (core/renderer/html.go renderMediaElement),
// con una diferencia importante: acá se usa renderer.ValidateURLScheme, NO
// SanitizeURL. SanitizeURL aplica EscapeHTMLAttribute encima del allowlist
// de esquema — correcto para interpolar en un atributo HTML, pero el
// target del hyperlink de DOCX y el texto visible no son HTML, así que esa
// escapada solo mete entidades (`&amp;`) literales en una URL con query
// string. Un source vacío se distingue de uno bloqueado con mensajes
// distintos.
func (g *DOCXGenerator) renderMedia(doc domain.Document, elem *ast.MediaElement) error {
	label := "video"
	if elem.MediaType == "audio" {
		label = "audio"
	}

	source := strings.TrimSpace(elem.Source)
	if source == "" {
		return g.renderPlaceholder(doc, fmt.Sprintf("%s sin fuente", label))
	}
	safeSource := renderer.ValidateURLScheme(source)
	if safeSource == "" {
		g.logger.Warn("DOCX: Media source blocked (dangerous scheme): %s", source)
		return g.renderPlaceholder(doc, fmt.Sprintf("%s bloqueado por seguridad", label))
	}

	p, err := doc.AddParagraph()
	if err != nil {
		return err
	}
	if err := p.SetAlignment(domain.AlignmentCenter); err != nil {
		return fmt.Errorf("invalid alignment: %w", err)
	}

	displayText := fmt.Sprintf("[%s: %s]", label, safeSource)
	if _, err := p.AddHyperlink(safeSource, displayText); err != nil {
		g.logger.Warn("DOCX: Failed to insert media hyperlink %s: %v", safeSource, err)
		return g.renderPlaceholder(doc, fmt.Sprintf("%s not found: %s", label, safeSource))
	}

	return nil
}

// needsChromiumRendering verifica si necesitamos Chromium
func (g *DOCXGenerator) needsChromiumRendering(astDoc *ast.AST) bool {
	for _, block := range astDoc.ContentBlocks {
		for _, elem := range block.Elements {
			switch elem.(type) {
			// MathElement faltaba acá: renderMath ya usa chromiumRenderer
			// (RenderMathToPNG) pero, sin este case, un documento cuyo
			// único elemento rico es una ecuación nunca inicializa
			// chromiumRenderer, así que renderMath cae en su guarda de nil
			// y la ecuación desaparece del DOCX en silencio — math estaba
			// soportado solo nominalmente.
			case *ast.ChartElement, *ast.MapElement, *ast.MermaidElement, *ast.MathElement:
				return true
			}
		}
	}
	return false
}
