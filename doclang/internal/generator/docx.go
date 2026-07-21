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
	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/renderer"
	"go.ziradocs.com/core/renderer/chromium"
	"go.ziradocs.com/core/util"
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
			g.logger.Warn("DOCX", "Failed to initialize Chromium: %v", err)
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
	}

	// Renderizar frontmatter (título del documento)
	if err := g.renderFrontMatter(doc, astDoc.FrontMatter); err != nil {
		return fmt.Errorf("error rendering frontmatter: %w", err)
	}

	// Recolectar encabezados para TOC
	tocEntries := g.collectHeadings(astDoc)

	// Renderizar TOC
	if err := g.renderTOC(doc, tocEntries); err != nil {
		return fmt.Errorf("error rendering TOC: %w", err)
	}

	// Renderizar secciones del documento
	for i := range astDoc.ContentBlocks {
		if err := g.renderSection(doc, &astDoc.ContentBlocks[i]); err != nil {
			return fmt.Errorf("error rendering section: %w", err)
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

// collectHeadings recolecta todos los encabezados del documento para el TOC
func (g *DOCXGenerator) collectHeadings(astDoc *ast.AST) []TOCEntry {
	var entries []TOCEntry

	g.logger.Info("DOCX", "📋 Collecting headings from %d content blocks", len(astDoc.ContentBlocks))

	for _, block := range astDoc.ContentBlocks {
		// H1 (section title)
		if block.Title != "" {
			entries = append(entries, TOCEntry{
				Title:      block.Title,
				Level:      1,
				BookmarkID: sanitizeBookmarkID(block.Title),
			})
			g.logger.Info("DOCX", "  ➜ H1: %s", block.Title)
		}

		// H2/H3/H4 (text elements with raw HTML or markdown headers)
		for _, elem := range block.Elements {
			switch typedElem := elem.(type) {
			case *ast.TextElement:
				content := typedElem.Content

				// Check for raw HTML headers (IsRawHTML=true)
				if typedElem.IsRawHTML {
					if h2Match := regexp.MustCompile(`<h2[^>]*>(.+?)</h2>`).FindStringSubmatch(content); h2Match != nil {
						entries = append(entries, TOCEntry{
							Title:      h2Match[1],
							Level:      2,
							BookmarkID: sanitizeBookmarkID(h2Match[1]),
						})
						g.logger.Info("DOCX", "  ➜ H2: %s", h2Match[1])
					} else if h3Match := regexp.MustCompile(`<h3[^>]*>(.+?)</h3>`).FindStringSubmatch(content); h3Match != nil {
						entries = append(entries, TOCEntry{
							Title:      h3Match[1],
							Level:      3,
							BookmarkID: sanitizeBookmarkID(h3Match[1]),
						})
						g.logger.Info("DOCX", "  ➜ H3: %s", h3Match[1])
					} else if h4Match := regexp.MustCompile(`<h4[^>]*>(.+?)</h4>`).FindStringSubmatch(content); h4Match != nil {
						entries = append(entries, TOCEntry{
							Title:      h4Match[1],
							Level:      4,
							BookmarkID: sanitizeBookmarkID(h4Match[1]),
						})
						g.logger.Info("DOCX", "  ➜ H4: %s", h4Match[1])
					}
				} else {
					// Check for markdown headers (## text)
					if h2Match := regexp.MustCompile(`^## (.+)`).FindStringSubmatch(content); h2Match != nil {
						entries = append(entries, TOCEntry{
							Title:      h2Match[1],
							Level:      2,
							BookmarkID: sanitizeBookmarkID(h2Match[1]),
						})
						g.logger.Info("DOCX", "  ➜ H2: %s", h2Match[1])
					} else if h3Match := regexp.MustCompile(`^### (.+)`).FindStringSubmatch(content); h3Match != nil {
						entries = append(entries, TOCEntry{
							Title:      h3Match[1],
							Level:      3,
							BookmarkID: sanitizeBookmarkID(h3Match[1]),
						})
						g.logger.Info("DOCX", "  ➜ H3: %s", h3Match[1])
					} else if h4Match := regexp.MustCompile(`^#### (.+)`).FindStringSubmatch(content); h4Match != nil {
						entries = append(entries, TOCEntry{
							Title:      h4Match[1],
							Level:      4,
							BookmarkID: sanitizeBookmarkID(h4Match[1]),
						})
						g.logger.Info("DOCX", "  ➜ H4: %s", h4Match[1])
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
							entries = append(entries, TOCEntry{
								Title:      m[1],
								Level:      hp.level,
								BookmarkID: sanitizeBookmarkID(m[1]),
							})
							g.logger.Info("DOCX", "  ➜ H%d (grid column): %s", hp.level, m[1])
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

	// Crear field TOC usando el constructor con switches
	tocSwitches := map[string]string{
		"o": "1-3", // niveles de outline 1-3 (H1, H2, H3)
		"h": "",    // crear hyperlinks
		"z": "",    // ocultar números de página en vista web
		"u": "",    // usar niveles de outline
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

// renderSection renderiza una sección del documento (H1 + elementos)
func (g *DOCXGenerator) renderSection(doc domain.Document, section *ast.ContentBlock) error {
	// Título de la sección (H1)
	if section.Title != "" {
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

		r, err := p.AddRun()
		if err != nil {
			return err
		}
		_ = r.SetText(section.Title)
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
	default:
		g.logger.Warn("DOCX", "Unknown element type: %T", elem)
		return nil
	}
}

// renderText renderiza texto/párrafos/headings
func (g *DOCXGenerator) renderText(doc domain.Document, elem *ast.TextElement) error {
	content := elem.Content

	// Detectar headings HTML (del FlexParser)
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
	}

	_ = p.SetStyle(styleID)
	if err := p.SetSpacingBefore(g.parseTwips(spaceBefore)); err != nil {
		return fmt.Errorf("invalid spacing before: %w", err)
	}
	if err := p.SetSpacingAfter(g.parseTwips(spaceAfter)); err != nil {
		return fmt.Errorf("invalid spacing after: %w", err)
	}

	r, err := p.AddRun()
	if err != nil {
		return err
	}
	_ = r.SetText(text)
	if err := r.SetSize(g.parseSize(size)); err != nil {
		return fmt.Errorf("invalid font size: %w", err)
	}
	_ = r.SetColor(g.parseColor(color))
	_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
	if bold {
		_ = r.SetBold(true)
	}

	// TODO: Agregar bookmark

	return nil
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
	// Regex patterns para markdown inline
	// Orden: code, bold, italic, links (code primero para evitar procesar ** dentro de `)
	patterns := []struct {
		regex *regexp.Regexp
		apply func(domain.Run, string) error
	}{
		{
			// `code` - código inline
			regex: regexp.MustCompile("`([^`]+)`"),
			apply: func(r domain.Run, text string) error {
				_ = r.SetText(text)
				if err := r.SetSize(g.parseSize(g.style.FontSizeCode)); err != nil {
					return err
				}
				_ = r.SetColor(g.parseColor(g.style.CodeInlineColor))
				_ = r.SetFont(domain.Font{Name: g.style.CodeFontFamily})
				return nil
			},
		},
		{
			// **bold** - negrita
			regex: regexp.MustCompile(`\*\*([^*]+)\*\*`),
			apply: func(r domain.Run, text string) error {
				_ = r.SetText(text)
				if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
					return err
				}
				_ = r.SetColor(g.parseColor(g.style.TextColor))
				_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
				_ = r.SetBold(true)
				return nil
			},
		},
		{
			// *italic* - cursiva
			regex: regexp.MustCompile(`\*([^*]+)\*`),
			apply: func(r domain.Run, text string) error {
				_ = r.SetText(text)
				if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
					return err
				}
				_ = r.SetColor(g.parseColor(g.style.TextColor))
				_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
				_ = r.SetItalic(true)
				return nil
			},
		},
		{
			// [text](url) - links
			regex: regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`),
			apply: func(r domain.Run, text string) error {
				_ = r.SetText(text)
				if err := r.SetSize(g.parseSize(g.style.FontSizeBase)); err != nil {
					return err
				}
				_ = r.SetColor(g.parseColor(g.style.LinkColor))
				_ = r.SetFont(domain.Font{Name: g.style.FontFamily})
				// Links con subrayado (usar UnderlineNone + 1 = single)
				_ = r.SetUnderline(domain.UnderlineStyle(1))
				// TODO: Agregar hyperlink real cuando docxgo lo soporte
				return nil
			},
		},
	}

	// Parser simple: procesar segmento por segmento
	remaining := content
	pos := 0

	for pos < len(remaining) {
		// Buscar el próximo match de cualquier pattern
		minPos := len(remaining)
		var matchedPattern *struct {
			regex *regexp.Regexp
			apply func(domain.Run, string) error
		}
		var matchedText string
		var matchedInner string

		for i := range patterns {
			pattern := &patterns[i]
			loc := pattern.regex.FindStringSubmatchIndex(remaining[pos:])
			if loc != nil && loc[0] < minPos {
				minPos = loc[0]
				matchedPattern = pattern
				matchedText = remaining[pos+loc[0] : pos+loc[1]]
				if len(loc) >= 4 {
					matchedInner = remaining[pos+loc[2] : pos+loc[3]]
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
			pos += minPos
		}

		// Agregar texto con formato
		if matchedPattern != nil {
			r, err := p.AddRun()
			if err != nil {
				return err
			}
			if err := matchedPattern.apply(r, matchedInner); err != nil {
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
			g.logger.Warn("DOCX", "Image source blocked (outside asset root): %s: %v", imagePath, err)
			return g.renderImagePlaceholder(p, fmt.Sprintf("[Image blocked: %s]", imagePath))
		}
		imagePath = confined
	}

	// Tamaño por defecto: 6 pulgadas de ancho (mantiene aspect ratio)
	imageSize := domain.NewImageSizeInches(6.0, 0) // 0 = mantener proporción

	// Agregar imagen
	img, err := p.AddImageWithSize(imagePath, imageSize)
	if err != nil {
		g.logger.Warn("DOCX", "Failed to insert image %s: %v", imagePath, err)
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
		g.logger.Warn("DOCX", "Chromium not available, skipping mermaid diagram")
		return nil
	}

	g.logger.Info("DOCX", "Rendering Mermaid diagram (%s)...", elem.DiagramType)

	// Renderizar a PNG usando ChromiumRenderer con mayor resolución
	// Usar dimensiones más grandes para que Mermaid tenga más espacio
	pngBytes, err := g.chromiumRenderer.RenderMermaidToPNG(context.Background(), elem.Content, 2400, 1600)
	if err != nil {
		g.logger.Warn("DOCX", "Failed to render mermaid: %v", err)
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
		g.logger.Warn("DOCX", "Failed to insert mermaid image: %v", err)
		return g.renderPlaceholder(doc, fmt.Sprintf("Mermaid Diagram: %s", elem.DiagramType))
	}

	if img == nil {
		g.logger.Warn("DOCX", "⚠️  Mermaid image object is nil after insertion")
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
		g.logger.Warn("DOCX", "Chromium not available, skipping equation")
		return nil
	}

	g.logger.Info("DOCX", "Rendering equation...")

	pngBytes, err := g.chromiumRenderer.RenderMathToPNG(context.Background(), elem.Content, 1600, 400)
	if err != nil {
		g.logger.Warn("DOCX", "Failed to render equation: %v", err)
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
		g.logger.Warn("DOCX", "Failed to insert equation image: %v", err)
		return g.renderPlaceholder(doc, "Equation")
	}
	if img == nil {
		g.logger.Warn("DOCX", "⚠️  Equation image object is nil after insertion")
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
		g.logger.Warn("DOCX", "Chromium not available, skipping chart")
		return nil
	}

	g.logger.Info("DOCX", "Rendering Chart.js chart (%s)...", elem.ChartType)

	// Generar configuración de Chart.js optimizada para exportación a PNG
	chartConfig := renderer.GenerateChartConfigForExport(elem)

	// Renderizar a PNG usando ChromiumRenderer con alta resolución para mejor calidad en Word
	// 2400x1500 pixels = buena calidad para impresión y pantalla
	pngBytes, err := g.chromiumRenderer.RenderChartToPNG(context.Background(), chartConfig, 2400, 1500)
	if err != nil {
		g.logger.Warn("DOCX", "Failed to render chart: %v", err)
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
		g.logger.Warn("DOCX", "Failed to insert chart image: %v", err)
		return g.renderPlaceholder(doc, fmt.Sprintf("Chart: %s", elem.ChartType))
	}

	if img == nil {
		g.logger.Warn("DOCX", "⚠️  Image object is nil after insertion")
	} else {
		g.logger.Debug("DOCX", "Image object created successfully")
	}

	sizeKB := float64(len(pngBytes)) / 1024
	g.logger.Info("DOCX", "✅ Chart inserted (%.1f KB)", sizeKB)

	return nil
}

func (g *DOCXGenerator) renderMap(doc domain.Document, elem *ast.MapElement) error {
	if g.chromiumRenderer == nil {
		g.logger.Warn("DOCX", "Chromium not available, skipping map")
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
		g.logger.Warn("DOCX", "Failed to render map: %v", err)
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
		g.logger.Warn("DOCX", "Failed to insert map image: %v", err)
		return g.renderPlaceholder(doc, fmt.Sprintf("Map: %s", elem.MapType))
	}

	if img == nil {
		g.logger.Warn("DOCX", "⚠️  Map image object is nil after insertion")
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
		g.logger.Warn("DOCX", "Failed to fetch PlantUML diagram: %v", err)
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
		g.logger.Warn("DOCX", "Failed to insert PlantUML image: %v", err)
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

// needsChromiumRendering verifica si necesitamos Chromium
func (g *DOCXGenerator) needsChromiumRendering(astDoc *ast.AST) bool {
	for _, block := range astDoc.ContentBlocks {
		for _, elem := range block.Elements {
			switch elem.(type) {
			case *ast.ChartElement, *ast.MapElement, *ast.MermaidElement:
				return true
			}
		}
	}
	return false
}
