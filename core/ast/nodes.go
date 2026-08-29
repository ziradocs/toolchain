// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"encoding/json"

	"go.ziradocs.com/core/v2/diagnostics"
)

// Los structs de configuración de este archivo (HeaderFooterConfig y su
// familia, TOCConfig, PageConfig, PageMargins, WatermarkConfig) llevan un tag
// `yaml:` que ESPEJA exactamente su tag `json:` — mismo nombre de llave, misma
// presencia o ausencia de omitempty. No es decoración: core/formatter emite el
// frontmatter con yaml.Marshal sobre estos structs (issue #230), y
// go.yaml.in/yaml/v3 lee solo el tag `yaml:` (el fallback de "tag desnudo" no
// aplica: un tag `json:"..."` contiene ':'), así que sin él cae a
// strings.ToLower(field.Name) y emite `excludetitleslides`/`startfrom`/
// `pagenumbers`/`layoutdefaults`/`fontsize` — llaves que parser/frontmatter.go
// no lee.
//
// Copiar la AUSENCIA de omitempty es lo que sostiene el tri-estado, no un
// descuido: WatermarkConfig.Enabled y los Enabled de Header/Footer/PageNumbers/
// Border son bools planos. Si `enabled: false` se omitiera en watermark, el
// reparse lo devolvería como true (convertWatermark arranca en Enabled: true
// cuando la llave está declarada). Los punteros con omitempty sí se emiten
// apuntando a su valor cero — yaml.v3 considera zero un puntero solo si es nil.
//
// FrontMatterNode NO se taguea a propósito: nunca se marshalea entero (Raw,
// BaseNode, y el hecho de que HeaderFooter se parta en tres llaves de nivel
// superior lo hacen imposible), y taguearlo insinuaría lo contrario.

// HeaderFooterConfig representa la configuración global de headers y footers
type HeaderFooterConfig struct {
	Header         *HeaderConfig                        `json:"header,omitempty" yaml:"header,omitempty"`
	Footer         *FooterConfig                        `json:"footer,omitempty" yaml:"footer,omitempty"`
	LayoutDefaults map[string]*LayoutHeaderFooterConfig `json:"layout_defaults,omitempty" yaml:"layout_defaults,omitempty"`
}

// HeaderConfig configura la apariencia y contenido del header
type HeaderConfig struct {
	Enabled    bool              `json:"enabled" yaml:"enabled"`
	Height     string            `json:"height,omitempty" yaml:"height,omitempty"`         // e.g., "60px", "4rem"
	Background string            `json:"background,omitempty" yaml:"background,omitempty"` // color o gradiente
	Text       *HeaderFooterText `json:"text,omitempty" yaml:"text,omitempty"`
	Logo       *LogoConfig       `json:"logo,omitempty" yaml:"logo,omitempty"`
	Border     *BorderConfig     `json:"border,omitempty" yaml:"border,omitempty"`
}

// FooterConfig configura la apariencia y contenido del footer
type FooterConfig struct {
	Enabled     bool               `json:"enabled" yaml:"enabled"`
	Height      string             `json:"height,omitempty" yaml:"height,omitempty"`         // e.g., "40px", "3rem"
	Background  string             `json:"background,omitempty" yaml:"background,omitempty"` // color o gradiente
	Text        *HeaderFooterText  `json:"text,omitempty" yaml:"text,omitempty"`
	PageNumbers *PageNumbersConfig `json:"page_numbers,omitempty" yaml:"page_numbers,omitempty"`
	Border      *BorderConfig      `json:"border,omitempty" yaml:"border,omitempty"`
}

// HeaderFooterText define el contenido de texto en headers/footers
type HeaderFooterText struct {
	Left   string `json:"left,omitempty" yaml:"left,omitempty"`
	Center string `json:"center,omitempty" yaml:"center,omitempty"`
	Right  string `json:"right,omitempty" yaml:"right,omitempty"`
}

// PageNumbersConfig configura la numeración de páginas
type PageNumbersConfig struct {
	Enabled              bool   `json:"enabled" yaml:"enabled"`
	Format               string `json:"format,omitempty" yaml:"format,omitempty"`     // e.g., "{{current}} / {{total}}", "Página {{current}}"
	Position             string `json:"position,omitempty" yaml:"position,omitempty"` // "left", "center", "right"
	ExcludeTitleSlides   bool   `json:"exclude_title_slides,omitempty" yaml:"exclude_title_slides,omitempty"`
	ExcludeClosingSlides bool   `json:"exclude_closing_slides,omitempty" yaml:"exclude_closing_slides,omitempty"`
	StartFrom            int    `json:"start_from,omitempty" yaml:"start_from,omitempty"`
	Style                string `json:"style,omitempty" yaml:"style,omitempty"` // "normal", "caption", "bold"
}

// LogoConfig configura logos en headers
type LogoConfig struct {
	Source   string `json:"source,omitempty" yaml:"source,omitempty"`     // ruta al logo
	Alt      string `json:"alt,omitempty" yaml:"alt,omitempty"`           // texto alternativo
	Height   string `json:"height,omitempty" yaml:"height,omitempty"`     // altura del logo
	Position string `json:"position,omitempty" yaml:"position,omitempty"` // "left", "center", "right"
}

// BorderConfig configura bordes en headers/footers
type BorderConfig struct {
	Enabled  bool   `json:"enabled" yaml:"enabled"`
	Color    string `json:"color,omitempty" yaml:"color,omitempty"`
	Width    string `json:"width,omitempty" yaml:"width,omitempty"`       // e.g., "1px", "2px"
	Style    string `json:"style,omitempty" yaml:"style,omitempty"`       // "solid", "dashed", "dotted"
	Position string `json:"position,omitempty" yaml:"position,omitempty"` // "top", "bottom", "both"
}

// LayoutHeaderFooterConfig permite overrides por tipo de layout
type LayoutHeaderFooterConfig struct {
	Header *HeaderConfig `json:"header,omitempty" yaml:"header,omitempty"`
	Footer *FooterConfig `json:"footer,omitempty" yaml:"footer,omitempty"`
}

// ContentBlockHeaderFooterOverride permite overrides por bloque de contenido individual
type ContentBlockHeaderFooterOverride struct {
	Header *HeaderConfig `json:"header,omitempty" yaml:"header,omitempty"`
	Footer *FooterConfig `json:"footer,omitempty" yaml:"footer,omitempty"`
}

// FrontMatterNode contiene el YAML parseado del FrontMatter
type FrontMatterNode struct {
	BaseNode `tstype:",extends,required"`
	Mode     string `json:"mode"`
	Title    string `json:"title,omitempty"`
	Author   string `json:"author,omitempty"`
	Date     string `json:"date,omitempty"`
	Theme    string `json:"theme,omitempty"`
	// Lang es el idioma principal declarado del documento, como tag BCP 47
	// (p.ej. "es", "en-US") — issue #62/#63: campo de primera clase para que
	// un renderer emita `<html lang>`/`w:lang` real. Deliberadamente NO se
	// refleja en Variables["lang"] (ver FrontMatterNode.BuildVariables):
	// ese mapa es de sustitución de placeholders en prosa, no de metadata
	// de documento, y promover "lang" ahí reescribiría silenciosamente
	// "{{lang}}" como texto literal en cualquier documento que lo declare.
	// Consecuencia (code review de este cambio): una regla de linter que
	// hoy lee FrontMatter.Variables["lang"] (p.ej. A11Y005/LangDeclaredRule
	// en enterprise) NO ve este campo — debe leer FrontMatter.Lang
	// directamente. Sintaxis, no semántica: ver a11y.IsValidLangTag para la
	// validación de forma (y su alcance: no cubre `privateuse`/
	// `grandfathered`).
	Lang         string                 `json:"lang,omitempty"`
	Variables    map[string]interface{} `json:"variables,omitempty"`
	HeaderFooter *HeaderFooterConfig    `json:"header_footer,omitempty"` // Nueva configuración
	// Numbering is a tri-state override for section auto-numbering
	// (`--numbering`): nil means the document has no opinion and the CLI's
	// own default applies; a non-nil pointer is an explicit `numbering:
	// true`/`numbering: false` in front matter. A plain bool couldn't
	// represent "unset" (its zero value is indistinguishable from an
	// explicit `numbering: false`), which is exactly what the CLI's
	// defaulting logic needs to tell apart from "document didn't say"
	// (doclang/internal/cli/build.go: an explicit `--numbering` flag still
	// wins over either). Needed because there was previously no
	// document-level way to opt out of numbering short of passing
	// `--numbering=false` on every build invocation — e.g. a document whose
	// section titles already carry their own numbering in the heading text.
	Numbering *bool `json:"numbering,omitempty"`
	// TOC and Page are namespaces (`toc:`, `page:` in front matter), not
	// plain scalars like Numbering — hence structs, not a top-level field
	// each. A flat `TOCDepth` field here would advertise a `toc_depth:` key
	// that does not exist in the YAML shape; keeping the namespace as a
	// struct mirrors HeaderFooter above and leaves room for an additive
	// sibling (`toc.title`, `page.orientation`) later without a contract
	// change. See TOCConfig/PageConfig for why their sub-fields are also
	// pointers/raw strings rather than resolved values.
	TOC  *TOCConfig  `json:"toc,omitempty"`
	Page *PageConfig `json:"page,omitempty"`
	// Watermark is the parsed `watermark:` front matter namespace (issue
	// #179) — a repeating, semi-transparent overlay drawn behind content
	// on every slide/page. Same tri-state-pointer discipline as
	// Numbering/TOC: "not declared" must stay distinguishable from a field
	// declared at its zero value.
	Watermark *WatermarkConfig `json:"watermark,omitempty"`
	Raw       string           `json:"-"` // YAML crudo
}

// TOCConfig is the parsed `toc:` front matter namespace. Both fields are
// pointers for the same tri-state reason as FrontMatterNode.Numbering: the
// consumer's default is `true`/a positive depth, not the Go zero value, so
// "not declared" must be distinguishable from "declared false"/"declared
// zero". They are independent on purpose — `toc: true` (the scalar
// shorthand for "enabled, no opinion on depth") must not imply anything
// about Depth.
type TOCConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Depth   *int  `json:"depth,omitempty" yaml:"depth,omitempty"`
}

// PageConfig is the parsed `page:` front matter namespace. Size and the
// PageMargins fields are the author's raw text (`"A4"`, `"2cm"`) verbatim,
// NOT resolved to a concrete unit — this AST is a public JSON contract
// (schema/ast.schema.json, ast-types), and baking in one renderer's unit
// approximation (Chromium's inches, a future DOCX target's twips) would be
// both a lossy conversion and a loss of information an external consumer
// might want (e.g. a `--filter` reporting "size: Carta does not exist").
// See core/util/length.go for the shared resolver every consumer should use
// instead of re-parsing these strings ad hoc.
type PageConfig struct {
	Size    string       `json:"size,omitempty" yaml:"size,omitempty"`
	Margins *PageMargins `json:"margins,omitempty" yaml:"margins,omitempty"`
}

// PageMargins holds one raw length string per side. `margins: 2cm` (the
// scalar shorthand) fills all four; the per-side map form fills only what
// was declared, leaving the rest "" so the consumer falls back to its own
// per-side default instead of an all-or-nothing default. Deliberately not a
// 1/2/4-value CSS shorthand: nothing emits that today, and adding it later
// is a parser change, not a contract change.
type PageMargins struct {
	Top    string `json:"top,omitempty" yaml:"top,omitempty"`
	Right  string `json:"right,omitempty" yaml:"right,omitempty"`
	Bottom string `json:"bottom,omitempty" yaml:"bottom,omitempty"`
	Left   string `json:"left,omitempty" yaml:"left,omitempty"`
}

// WatermarkConfig is the parsed `watermark:` front matter namespace (issue
// #179): a repeating, semi-transparent text overlay rendered behind
// content on every slide (slidelang) or page (doclang). All fields besides
// Enabled/Text are optional pointers so a consumer can tell "author didn't
// say" apart from "author said the zero value" and apply its own default —
// same tri-state discipline as TOCConfig. FontSize is stored verbatim
// (`"72pt"`, `"2cm"`), never pre-resolved, for the same public-JSON-contract
// reason PageConfig.Size is: see core/util/length.go for the shared
// resolver every consumer should use instead of re-parsing it ad hoc.
type WatermarkConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
	// Text passes through the same {{variable}} substitution as
	// header/footer text (renderer.ProcessVariables) — an author can write
	// `watermark: "{{title}} — BORRADOR"`.
	Text     string   `json:"text,omitempty" yaml:"text,omitempty"`
	Color    string   `json:"color,omitempty" yaml:"color,omitempty"`
	Opacity  *float64 `json:"opacity,omitempty" yaml:"opacity,omitempty"`   // 0.0-1.0
	Rotation *float64 `json:"rotation,omitempty" yaml:"rotation,omitempty"` // degrees, clockwise
	FontSize string   `json:"font_size,omitempty" yaml:"font_size,omitempty"`
	Repeat   *bool    `json:"repeat,omitempty" yaml:"repeat,omitempty"`
}

// NewFrontMatterNode crea un nuevo nodo de FrontMatter
func NewFrontMatterNode(pos diagnostics.Position) *FrontMatterNode {
	return &FrontMatterNode{
		BaseNode:  NewBaseNode(NodeTypeFrontMatter, pos),
		Variables: make(map[string]interface{}),
	}
}

// BuildVariables arma el mapa de sustitución de `{{variable}}` para este
// FrontMatter: los built-ins documentados (title/author/date/theme) seguidos
// de las variables personalizadas del usuario. Compartido entre slidelang
// (slidelang/internal/generator/data.BuildVariables, que delega acá) y
// doclang (renderer/document_html.go) — issue #81: antes solo slidelang
// exponía los built-ins, así que `{{title}}` no se sustituía en el HTML de
// doclang.
func (fm *FrontMatterNode) BuildVariables() map[string]interface{} {
	if fm == nil {
		return nil
	}

	variables := make(map[string]interface{})

	if fm.Title != "" {
		variables["title"] = fm.Title
	}
	if fm.Author != "" {
		variables["author"] = fm.Author
	}
	if fm.Date != "" {
		variables["date"] = fm.Date
	}
	if fm.Theme != "" {
		variables["theme"] = fm.Theme
	}
	// Lang deliberadamente NO se agrega a los built-ins de sustitución: a
	// diferencia de title/author/date/theme, "lang" es una palabra común en
	// prosa que documenta el propio mecanismo de idioma (tutoriales, este
	// mismo repo) — convertir `{{lang}}` en un placeholder activo reescribe
	// silenciosamente ese texto literal en cualquier documento que declare
	// `lang:` (code review de este cambio). Nada depende de esto: A11Y005
	// lee FrontMatter.Variables["lang"] directamente (poblado por el
	// parser si el autor lo declara dentro de `variables:`), no este mapa.

	for k, v := range fm.Variables {
		variables[k] = v
	}

	return variables
}

// ContentBlock representa un bloque de contenido (slide en presentaciones, sección en documentos)
//
// Title y Heading tienen un split de responsabilidad que no está escrito en
// ningún otro lugar (issue #52): en documentos, el PRIMER ContentBlock del
// AST (blockType "title", el bloque de preámbulo — el que trae el título
// del documento) guarda su texto en Heading; TODOS los demás bloques
// ("content") lo guardan en Title. slidelang usa el mismo split pero con
// otro criterio: un slide con `# ` (sintaxis flex) va a Heading
// independientemente de su posición; `## ` va a Title. El parser strict de
// ambos dialectos deja que el autor declare cualquiera de los dos por
// nombre (`title:`/`heading:` como propiedades), así que un bloque puede
// tener AMBOS campos poblados a la vez — ver SectionTitle para el
// desempate. Cada consumidor que necesita "el título que se muestra de este
// bloque" debe llamar SectionTitle() en vez de leer Title/Heading
// directamente — antes de este método, esa resolución estaba hand-rolled
// (y a veces en desacuerdo entre sí) en al menos core/renderer y doclang.
type ContentBlock struct {
	BaseNode  `tstype:",extends,required"`
	BlockType string `json:"blockType,omitempty"` // "title", "content", "section", etc.
	Title     string `json:"title,omitempty"`
	TitleHTML string `json:"titleHTML,omitempty"` // Title con {{variables}} sustituidas y escapadas (sin markdown)
	// Propiedades específicas para bloques tipo "title" (usado en presentaciones)
	Heading      string `json:"heading,omitempty"`
	HeadingHTML  string `json:"headingHTML,omitempty"` // Heading con {{variables}} sustituidas y escapadas (sin markdown)
	Subtitle     string `json:"subtitle,omitempty"`
	SubtitleHTML string `json:"subtitleHTML,omitempty"` // Subtitle con {{variables}} sustituidas y escapadas (sin markdown)
	Logo         string `json:"logo,omitempty"`
	// Elements está en orden de documento, y ese orden ES el contrato de
	// lectura/presentación (issue #62): todo renderer del repo (HTML, DOCX,
	// Markdown, PPTX) emite/recorre Elements en este mismo orden, sin
	// reordenar. Lo hacen cumplir TestGenerateDocumentHTML_PreservesElementOrder
	// (+ la variante _HeterogeneousTypes, core/renderer),
	// TestDOCXGenerator_PreservesElementOrder,
	// TestMarkdownGenerator_PreservesElementOrder (doclang/internal/generator)
	// y TestGeneratePPTX_PreservesElementOrder (slidelang/internal/generator).
	// Hoy ningún renderer tiene un mecanismo que desacople el orden visual
	// del orden de Elements — no hay float alcanzable (ver issue #72, el
	// único selector CSS de float nunca matchea código real), ni CSS
	// `order:`, ni override de grid-column/grid-row alcanzable desde input
	// real — así que este orden es también el orden visual efectivo. Si
	// algún día existe un mecanismo así, ese es el punto para agregar un
	// campo de AST que lo señale; no es especulativo hoy.
	Elements []Element `json:"elements"`
	// Configuración específica de header/footer para este bloque
	HeaderFooterOverride *ContentBlockHeaderFooterOverride `json:"header_footer_override,omitempty"`
}

// SectionTitle resuelve el título a mostrar de este ContentBlock y si
// cuenta como sección numerada — ver el split Title/Heading documentado en
// el doc comment del struct. Title gana cuando ambos están poblados (el
// dialecto strict lo permite, ver ContentBlock's doc comment): es lo que
// las dos rutas de render existentes (core/renderer/document_html.go,
// doclang/internal/generator/markdown.go — antes de este método, cada una
// con su propia copia hand-rolled de esta misma función) ya hacían, así que
// promoverlo a contrato compartido no les cambia el comportamiento. Un
// Title vacío es la señal de que este bloque es el preámbulo del documento
// (su texto vive en Heading en su lugar) — numerarlo como "1." y correr el
// resto de las secciones un número adelante es confuso (issue #100), así
// que numbered es false en ese caso.
func (c ContentBlock) SectionTitle() (title string, numbered bool) {
	if c.Title != "" {
		return c.Title, true
	}
	return c.Heading, false
}

// NewContentBlock crea un nuevo bloque de contenido
func NewContentBlock(pos diagnostics.Position, blockType string) *ContentBlock {
	return &ContentBlock{
		BaseNode:  NewBaseNode(NodeTypeContentBlock, pos),
		BlockType: blockType,
		Elements:  make([]Element, 0),
	}
}

// Element es una interfaz para elementos dentro de un bloque de contenido
type Element interface {
	Node
	element() // Método marcador
}

// TextElement representa un bloque de texto
type TextElement struct {
	BaseNode  `tstype:",extends,required"`
	Content   string `json:"content"`
	IsRawHTML bool   `json:"isRawHTML,omitempty"` // Si true, el contenido es HTML que no debe escaparse
	// ContentHTML es Content ya renderizado a HTML inline (markdown +
	// {{variables}} sustituidas y escapadas), idéntico al fragmento que
	// produce --format html para este elemento. Poblado por
	// renderer.PopulateInlineHTML (issue #64) para que consumidores de
	// --format json (p. ej. el viewer) no reimplementen el dialecto inline.
	ContentHTML string `json:"contentHTML,omitempty"`
	// Level is the heading level (1-6) when this TextElement represents a
	// `##`-`######` heading produced by DocumentFlexParser; 0 (omitted) for
	// regular text. Exposes the level as a first-class semantic field so a
	// linter rule (e.g. A11Y heading order/nesting, issue #22) can read it
	// without re-parsing the rendered `<hN>` in Content/IsRawHTML — a fragile
	// coupling to render format. Note: a document's H1 lives on
	// ContentBlock.Heading/Title (a string, not an element), not here; a
	// heading-order rule must treat that Heading as level 1 and walk Level
	// for the rest.
	Level int `json:"level,omitempty"`
	// LangRuns exposes [texto]{lang=xx} spans found in Content — see
	// LangRun's doc comment. Populated by renderer.PopulateLangRuns, always
	// re-derived from Content (never trusted from an external --filter, see
	// that function's doc comment), so it is NOT cleared by
	// ast.ClearRenderedHTML the way *HTML fields are — there is nothing
	// pre-rendered here to distrust, only something re-derived every time.
	LangRuns []LangRun `json:"langRuns,omitempty"`
	// DiscardedLangRuns is the mirror image of LangRuns (issue #92): every
	// [texto]{lang=xx} span found in Content whose tag failed
	// a11y.IsValidLangTag, so it never became a LangRuns entry. Without this,
	// a consumer that cares whether a language mark existed and didn't take
	// (e.g. a WCAG 3.1.2 linter rule, a formatter round-tripping content) had
	// no way to learn that from the populated AST — it had to independently
	// re-scan Content with its own copy of the extraction pattern to find
	// what renderer.PopulateLangRuns already found and silently discarded.
	// Same re-derivation/never-cleared rules as LangRuns.
	DiscardedLangRuns []LangRun `json:"discardedLangRuns,omitempty"`
}

func (t TextElement) element() {}

// NewTextElement crea un nuevo elemento de texto
func NewTextElement(pos diagnostics.Position, content string) *TextElement {
	return &TextElement{
		BaseNode:  NewBaseNode(NodeTypeText, pos),
		Content:   content,
		IsRawHTML: false,
	}
}

// NewRawHTMLTextElement crea un nuevo elemento de texto con HTML crudo
func NewRawHTMLTextElement(pos diagnostics.Position, htmlContent string) *TextElement {
	return &TextElement{
		BaseNode:  NewBaseNode(NodeTypeText, pos),
		Content:   htmlContent,
		IsRawHTML: true,
	}
}

// PointsElement representa una lista de puntos
type PointsElement struct {
	BaseNode `tstype:",extends,required"`
	Items    []PointItem `json:"items"`
	ListType string      `json:"listType"` // "ordered" para numeradas, "unordered" para bullets
}

func (p PointsElement) element() {}

// NewPointsElement crea un nuevo elemento de puntos
func NewPointsElement(pos diagnostics.Position) *PointsElement {
	return &PointsElement{
		BaseNode: NewBaseNode(NodeTypePoints, pos),
		Items:    make([]PointItem, 0),
		ListType: "unordered", // default a lista no ordenada
	}
}

// PointItem representa un item en una lista
type PointItem struct {
	BaseNode          `tstype:",extends,required"`
	Content           string      `json:"content"`
	ContentHTML       string      `json:"contentHTML,omitempty"`       // ver TextElement.ContentHTML
	LangRuns          []LangRun   `json:"langRuns,omitempty"`          // ver TextElement.LangRuns
	DiscardedLangRuns []LangRun   `json:"discardedLangRuns,omitempty"` // ver TextElement.DiscardedLangRuns
	SubPoints         []PointItem `json:"subPoints,omitempty"`
}

// NewPointItem crea un nuevo item de punto
func NewPointItem(pos diagnostics.Position, content string) *PointItem {
	return &PointItem{
		BaseNode:  NewBaseNode(NodeTypePointItem, pos),
		Content:   content,
		SubPoints: make([]PointItem, 0),
	}
}

// CodeElement representa un bloque de código
type CodeElement struct {
	BaseNode    `tstype:",extends,required"`
	Language    string `json:"language,omitempty"`
	Content     string `json:"content"`
	ContentHTML string `json:"contentHTML,omitempty"` // Content con {{variables}} sustituidas y escapado HTML (sin markdown, ver renderCodeElement)
}

func (c CodeElement) element() {}

// NewCodeElement crea un nuevo elemento de código
func NewCodeElement(pos diagnostics.Position, language, content string) *CodeElement {
	return &CodeElement{
		BaseNode: NewBaseNode(NodeTypeCode, pos),
		Language: language,
		Content:  content,
	}
}

// ImageContext representa el contexto de uso de una imagen
type ImageContext string

const (
	ImageContextTitle      ImageContext = "title"      // Para logos/imágenes en slides de título principal
	ImageContextHero       ImageContext = "hero"       // Para imágenes destacadas en slides de contenido
	ImageContextGallery    ImageContext = "gallery"    // Para múltiples imágenes agrupadas
	ImageContextContent    ImageContext = "content"    // Para imágenes integradas en texto
	ImageContextStandalone ImageContext = "standalone" // Para imágenes aisladas
)

// ImageElement representa una imagen
type ImageElement struct {
	BaseNode    `tstype:",extends,required"`
	Source      string       `json:"source"`
	Alt         string       `json:"alt,omitempty"`
	AltHTML     string       `json:"altHTML,omitempty"` // Alt con {{variables}} sustituidas y escapadas (sin markdown)
	Caption     string       `json:"caption,omitempty"`
	CaptionHTML string       `json:"captionHTML,omitempty"` // Caption con {{variables}} sustituidas y escapadas (sin markdown)
	Context     ImageContext `json:"context,omitempty"`
	// Label es el identificador de referencia cruzada del MVP OSS (issue
	// #239, decisión B), p. ej. "fig:arquitectura" — declarado como
	// `label:` junto a `caption:`. Number lo asigna el pase de numeración
	// (transform built-in sobre ast.Walk, ver core/xref) en orden
	// de documento; ninguno de los dos participa en *HTML (no son prosa).
	Label  string `json:"label,omitempty"`
	Number int    `json:"number,omitempty"`
}

func (i ImageElement) element() {}

// NewImageElement crea un nuevo elemento de imagen
func NewImageElement(pos diagnostics.Position, source, alt string) *ImageElement {
	return &ImageElement{
		BaseNode: NewBaseNode(NodeTypeImage, pos),
		Source:   source,
		Alt:      alt,
		Context:  ImageContextStandalone, // Default context
	}
}

// NewImageElementWithContext crea un nuevo elemento de imagen con contexto específico
func NewImageElementWithContext(pos diagnostics.Position, source, alt string, context ImageContext) *ImageElement {
	return &ImageElement{
		BaseNode: NewBaseNode(NodeTypeImage, pos),
		Source:   source,
		Alt:      alt,
		Context:  context,
	}
}

// TableCell represents a single table cell with cross-cutting structure
// (issue #20, A11Y): scope and colspan/rowspan, so a linter rule can inspect
// merged cells and their declared scope. Deliberately WITHOUT its own *HTML
// field: unlike TextElement/ImageElement/etc., cell content is processed
// inline at render time (ProcessTextWithVariablesAndMarkdownSecure, same as
// Headers/Rows today) — there's no need to populate/clear pre-rendered HTML
// for a --format json consumer, so Cells doesn't participate in
// populate_inline_html.go/clear_html.go.
type TableCell struct {
	Content string `json:"content"`
	// IsHeader marks a header cell (rendered as <th>, not <td>).
	IsHeader bool `json:"isHeader,omitempty"`
	// Scope is "row", "col", or "" (undeclared) — same vocabulary as the
	// HTML scope= attribute. Only meaningful on an IsHeader cell.
	Scope string `json:"scope,omitempty"`
	// ColSpan/RowSpan: 0 or 1 mean "no merge" (equivalent to an implicit
	// colspan="1"); >1 merges that many columns/rows.
	ColSpan int `json:"colSpan,omitempty"`
	RowSpan int `json:"rowSpan,omitempty"`
}

// LangRun exposes a sub-span of an element's own prose (issue #63) that the
// author marked as being in a different language than the document's
// declared FrontMatter.Lang — e.g. a French phrase inside otherwise-Spanish
// text — via the inline span [texto]{lang=xx} (see
// renderer.ProcessInlineMarkdownFormatsSecure). Lang is what a linter rule
// needs; Text is only there to say which sub-string it applies to.
//
// Text is the PLAIN TEXT of the passage, not a verbatim substring of the
// element's raw Content: renderer.PopulateLangRuns extracts a run from one
// of two different source shapes depending on IsRawHTML (ordinary Markdown
// Content vs. a TextElement already materialized to HTML at parse time —
// see extractLangRuns), and only the Markdown shape has a well-defined
// "before markdown is applied" state to copy verbatim from. The RawHTML
// shape does not — by the time PopulateLangRuns sees it, the span is already
// a literal <span lang="fr">a <strong>b</strong> c</span>, so Text is
// derived by stripping the HTML tags, not by finding some earlier
// pre-markdown source that no longer exists. Concretely: from Markdown
// Content, [a *b* c]{lang=fr} yields Text "a *b* c" (markdown syntax intact,
// unprocessed); from RawHTML Content, the equivalent already-rendered span
// yields Text "a b c" (HTML tags stripped, markdown syntax gone because it
// was already consumed at parse time). Do not assume Text is a substring of
// Content in either case.
//
// Deliberately WITHOUT BaseNode, same reasoning as TableCell: LangRuns is
// derived by renderer.PopulateLangRuns from Content, not walked by ast.Walk,
// so a diagnostic about a malformed or missing language mark can only point
// at the containing element's position, not at the run itself. Lang is
// always a11y.IsValidLangTag-valid by the time it lands here —
// PopulateLangRuns re-validates it on every extraction path (including a
// TextElement's already-rendered IsRawHTML content, which a hostile
// --filter could otherwise forge a fake <span lang> into) — so a rule can
// trust it without re-validating.
type LangRun struct {
	Text string `json:"text"`
	Lang string `json:"lang"`
}

// TableElement representa una tabla con datos
type TableElement struct {
	BaseNode    `tstype:",extends,required"`
	Headers     []string   `json:"headers"`
	HeadersHTML []string   `json:"headersHTML,omitempty"` // Headers ya renderizados a HTML inline (ver TextElement.ContentHTML)
	Rows        [][]string `json:"rows"`
	RowsHTML    [][]string `json:"rowsHTML,omitempty"` // Rows ya renderizadas a HTML inline (ver TextElement.ContentHTML)
	// Cells exposes the real cell structure (issue #20, A11Y: colspan/
	// rowspan/scope) IN ADDITION to Headers/Rows, never in their place —
	// Headers/Rows remain the source existing renderers and slidelang
	// consume for the simple case (additive, no compatibility break).
	// Populated by every table parser: TableParser.Parse derives Cells from
	// Headers/Rows for the simple case, or parses it directly from an
	// explicit YAML `cells:` block for merged cells — see
	// ast.DeriveCellsFromFlat. When Cells comes from the explicit `cells:`
	// block, Headers/Rows are DERIVED from Cells instead (expanding each
	// span into a rectangular grid), so linter.ElementStructureRule
	// (TABLE003: every row must have the same column count as Headers)
	// doesn't report a false positive on a table with merged cells.
	Cells       [][]TableCell `json:"cells,omitempty"`
	Caption     string        `json:"caption,omitempty"`
	CaptionHTML string        `json:"captionHTML,omitempty"` // Caption con {{variables}} sustituidas y escapadas (sin markdown)
	// Label/Number: ver ImageElement.Label/Number (mismo mecanismo de
	// referencia cruzada, issue #239).
	Label  string `json:"label,omitempty"`
	Number int    `json:"number,omitempty"`
}

func (t TableElement) element() {}

// NewTableElement crea un nuevo elemento de tabla
func NewTableElement(pos diagnostics.Position) *TableElement {
	return &TableElement{
		BaseNode: NewBaseNode(NodeTypeTable, pos),
		Headers:  make([]string, 0),
		Rows:     make([][]string, 0),
	}
}

// SpecialBlockElement representa bloques especiales (info, warning, etc.)
type SpecialBlockElement struct {
	BaseNode    `tstype:",extends,required"`
	BlockType   string `json:"blockType"` // "info", "warning", "danger", "success", "tip", "details"
	Title       string `json:"title,omitempty"`
	TitleHTML   string `json:"titleHTML,omitempty"` // Title con {{variables}} sustituidas y escapadas (sin markdown)
	Content     string `json:"content"`
	ContentHTML string `json:"contentHTML,omitempty"` // ver TextElement.ContentHTML
	// LangRuns cubre solo Content, no Title — mismo criterio de
	// campo-único que QuoteElement.LangRuns: una sola lista de runs no
	// podría decir de cuál de los dos campos vino cada uno, y Title suele
	// ser una etiqueta corta ("Nota", "Advertencia"), no prosa donde marcar
	// un idioma distinto tenga sentido. Ver TextElement.LangRuns.
	LangRuns []LangRun `json:"langRuns,omitempty"`
	// DiscardedLangRuns cubre solo Content, mismo alcance que LangRuns arriba.
	// Ver TextElement.DiscardedLangRuns.
	DiscardedLangRuns []LangRun `json:"discardedLangRuns,omitempty"`
	Icon              string    `json:"icon,omitempty"`
}

func (s SpecialBlockElement) element() {}

// NewSpecialBlockElement crea un nuevo bloque especial
func NewSpecialBlockElement(pos diagnostics.Position, blockType, content string) *SpecialBlockElement {
	return &SpecialBlockElement{
		BaseNode:  NewBaseNode(NodeTypeSpecialBlock, pos),
		BlockType: blockType,
		Content:   content,
	}
}

// CodeGroupElement representa un grupo de códigos con tabs
type CodeGroupElement struct {
	BaseNode   `tstype:",extends,required"`
	CodeBlocks []CodeBlock `json:"codeBlocks"`
}

func (c CodeGroupElement) element() {}

// CodeBlock representa un bloque individual en un grupo
type CodeBlock struct {
	Language    string `json:"language"`
	Label       string `json:"label"`
	LabelHTML   string `json:"labelHTML,omitempty"` // Label con {{variables}} sustituidas y escapado HTML (sin markdown, ver renderCodeGroupElement)
	Content     string `json:"content"`
	ContentHTML string `json:"contentHTML,omitempty"` // ver CodeElement.ContentHTML
}

// NewCodeGroupElement crea un nuevo grupo de códigos
func NewCodeGroupElement(pos diagnostics.Position) *CodeGroupElement {
	return &CodeGroupElement{
		BaseNode:   NewBaseNode(NodeTypeCodeGroup, pos),
		CodeBlocks: make([]CodeBlock, 0),
	}
}

// MermaidElement representa un diagrama Mermaid
type MermaidElement struct {
	BaseNode    `tstype:",extends,required"`
	DiagramType string `json:"diagramType"` // "graph", "sequence", "class", etc.
	Content     string `json:"content"`
	Title       string `json:"title,omitempty"`
	TitleHTML   string `json:"titleHTML,omitempty"` // Title con {{variables}} sustituidas y escapadas (sin markdown); Content es fuente de diagrama, no lleva *HTML (ver https://ziradocs.com/docs/architecture/json-ast-contract/)
}

func (m MermaidElement) element() {}

// NewMermaidElement crea un nuevo diagrama Mermaid
func NewMermaidElement(pos diagnostics.Position, diagramType, content string) *MermaidElement {
	return &MermaidElement{
		BaseNode:    NewBaseNode(NodeTypeMermaid, pos),
		DiagramType: diagramType,
		Content:     content,
	}
}

// PlantUMLElement representa un diagrama PlantUML
type PlantUMLElement struct {
	BaseNode    `tstype:",extends,required"`
	DiagramType string `json:"diagramType"` // "sequence", "class", "component", etc.
	Content     string `json:"content"`
	Title       string `json:"title,omitempty"`
	TitleHTML   string `json:"titleHTML,omitempty"` // ver MermaidElement.TitleHTML
}

func (p PlantUMLElement) element() {}

// NewPlantUMLElement crea un nuevo diagrama PlantUML
func NewPlantUMLElement(pos diagnostics.Position, diagramType, content string) *PlantUMLElement {
	return &PlantUMLElement{
		BaseNode:    NewBaseNode(NodeTypePlantUML, pos),
		DiagramType: diagramType,
		Content:     content,
	}
}

// ChartElement representa gráficos y charts
type ChartElement struct {
	BaseNode    `tstype:",extends,required"`
	ChartType   string                 `json:"chartType"`             // "bar", "line", "pie", "combo", etc.
	SeriesTypes []string               `json:"seriesTypes,omitempty"` // Para combo charts: ["bar", "bar", "line"]
	Data        [][]interface{}        `json:"data"`
	Series      []string               `json:"series,omitempty"`
	Labels      []string               `json:"labels,omitempty"` // Labels para los ejes
	Options     map[string]interface{} `json:"options,omitempty"`
	Title       string                 `json:"title,omitempty"`
	TitleHTML   string                 `json:"titleHTML,omitempty"`  // ver MermaidElement.TitleHTML; Data/Series/Labels/rawJSON son config de Chart.js, no llevan *HTML
	RawJSON     json.RawMessage        `json:"rawJSON,omitempty"`    // Para JSON directo; se serializa como objeto JSON anidado, no como string
	IsJSONMode  bool                   `json:"isJSONMode,omitempty"` // Indica si usa JSON directo
	Width       int                    `json:"width,omitempty"`      // Ancho personalizado (px), default 800
	Height      int                    `json:"height,omitempty"`     // Alto personalizado (px), default 600
}

func (c ChartElement) element() {}

// NewChartElement crea un nuevo elemento de gráfico
func NewChartElement(pos diagnostics.Position, chartType string) *ChartElement {
	return &ChartElement{
		BaseNode:    NewBaseNode(NodeTypeChart, pos),
		ChartType:   chartType,
		SeriesTypes: make([]string, 0),
		Data:        make([][]interface{}, 0),
		Series:      make([]string, 0),
		Labels:      make([]string, 0),
		Options:     make(map[string]interface{}),
	}
}

// MediaElement represents embedded audio/video content (issue #21, A11Y):
// exposes Autoplay/Controls/Loop as first-class fields so a linter rule can
// detect autoplay content with no pause/stop controls exposed to the user —
// something ast.Walk couldn't inspect before this type, because no media
// node existed at all.
//
// Note (a conscious decision, not an oversight): unlike ImageElement, this
// element carries no accessible-name field of its own (caption/track/
// aria-label) — the field list was followed as the issue requested it. It
// can be added in a future iteration if the A11Y rulepack needs it.
type MediaElement struct {
	BaseNode `tstype:",extends,required"`
	// MediaType is "video" or "audio" — determines the emitted HTML tag
	// (<video>/<audio>) and which authoring syntax produced it (<<video>>/<<audio>>).
	MediaType string `json:"mediaType"`
	Source    string `json:"source"`
	Autoplay  bool   `json:"autoplay,omitempty"`
	Controls  bool   `json:"controls,omitempty"`
	Loop      bool   `json:"loop,omitempty"`
	// Muted: autoplay without mute is blocked by most browsers, and
	// enabling autoplay with audio the user doesn't expect is itself a bad
	// A11Y practice — exposed as a separate field (not implied by Autoplay)
	// so a rule can require it explicitly.
	Muted bool `json:"muted,omitempty"`
}

func (m MediaElement) element() {}

// NewMediaElement creates a new media element
func NewMediaElement(pos diagnostics.Position, mediaType, source string) *MediaElement {
	return &MediaElement{
		BaseNode:  NewBaseNode(NodeTypeMedia, pos),
		MediaType: mediaType,
		Source:    source,
	}
}

// MapElement representa mapas con marcadores
type MapElement struct {
	BaseNode  `tstype:",extends,required"`
	MapType   string                 `json:"mapType"` // "world", "country", "region"
	Markers   []MapMarker            `json:"markers,omitempty"`
	Heatmap   bool                   `json:"heatmap,omitempty"`
	Zoom      int                    `json:"zoom,omitempty"`
	Center    *MapCoordinate         `json:"center,omitempty"`
	Title     string                 `json:"title,omitempty"`
	TitleHTML string                 `json:"titleHTML,omitempty"` // ver MermaidElement.TitleHTML; Markers son datos geográficos, no llevan *HTML
	Options   map[string]interface{} `json:"options,omitempty"`
	Width     int                    `json:"width,omitempty"`  // Ancho personalizado (px), default 800
	Height    int                    `json:"height,omitempty"` // Alto personalizado (px), default 600
}

func (m MapElement) element() {}

// MapMarker representa un marcador en el mapa
type MapMarker struct {
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Label   string  `json:"label"`
	Value   float64 `json:"value,omitempty"`
	Color   string  `json:"color,omitempty"`
	Size    string  `json:"size,omitempty"`
	Details string  `json:"details,omitempty"`
}

// MapCoordinate representa coordenadas del mapa
type MapCoordinate struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// NewMapElement crea un nuevo elemento de mapa
func NewMapElement(pos diagnostics.Position, mapType string) *MapElement {
	return &MapElement{
		BaseNode: NewBaseNode(NodeTypeMap, pos),
		MapType:  mapType,
		Markers:  make([]MapMarker, 0),
	}
}

// QuoteElement representa una cita en bloque
type QuoteElement struct {
	BaseNode    `tstype:",extends,required"`
	Content     string `json:"content"`
	ContentHTML string `json:"contentHTML,omitempty"` // ver TextElement.ContentHTML
	// LangRuns cubre solo Content — QuoteElement tiene tres campos de prosa
	// (Content/Author/Source) y una sola lista de runs no podría decir de
	// cuál vino cada uno; Content es la prosa citada, la que un pasaje en
	// otro idioma tiene sentido marcar (Author/Source suelen ser un nombre
	// propio, no oración). Ver TextElement.LangRuns.
	LangRuns []LangRun `json:"langRuns,omitempty"`
	// DiscardedLangRuns cubre solo Content, mismo alcance que LangRuns arriba.
	// Ver TextElement.DiscardedLangRuns.
	DiscardedLangRuns []LangRun `json:"discardedLangRuns,omitempty"`
	Author            string    `json:"author,omitempty"`     // Para citas con autor
	AuthorHTML        string    `json:"authorHTML,omitempty"` // Author con {{variables}} sustituidas y escapadas (sin markdown)
	Source            string    `json:"source,omitempty"`     // Para citas con fuente
	SourceHTML        string    `json:"sourceHTML,omitempty"` // Source con {{variables}} sustituidas y escapadas (sin markdown)
}

func (q QuoteElement) element() {}

// NewQuoteElement crea un nuevo elemento de cita
func NewQuoteElement(pos diagnostics.Position, content string) *QuoteElement {
	return &QuoteElement{
		BaseNode: NewBaseNode(NodeTypeQuote, pos),
		Content:  content,
	}
}

// ChecklistElement representa una lista de tareas con checkboxes
type ChecklistElement struct {
	BaseNode `tstype:",extends,required"`
	Items    []ChecklistItem `json:"items"`
}

func (c ChecklistElement) element() {}

// NewChecklistElement crea un nuevo elemento de checklist
func NewChecklistElement(pos diagnostics.Position) *ChecklistElement {
	return &ChecklistElement{
		BaseNode: NewBaseNode(NodeTypeChecklist, pos),
		Items:    make([]ChecklistItem, 0),
	}
}

// ChecklistItem representa un item en una lista de tareas
type ChecklistItem struct {
	BaseNode          `tstype:",extends,required"`
	Content           string          `json:"content"`
	ContentHTML       string          `json:"contentHTML,omitempty"`       // ver TextElement.ContentHTML
	LangRuns          []LangRun       `json:"langRuns,omitempty"`          // ver TextElement.LangRuns
	DiscardedLangRuns []LangRun       `json:"discardedLangRuns,omitempty"` // ver TextElement.DiscardedLangRuns
	Checked           bool            `json:"checked"`
	SubItems          []ChecklistItem `json:"subItems,omitempty"`
}

// NewChecklistItem crea un nuevo item de checklist
func NewChecklistItem(pos diagnostics.Position, content string, checked bool) *ChecklistItem {
	return &ChecklistItem{
		BaseNode: NewBaseNode(NodeTypeChecklistItem, pos),
		Content:  content,
		Checked:  checked,
		SubItems: make([]ChecklistItem, 0),
	}
}

// GridElement representa un contenedor de grid layout
type GridElement struct {
	BaseNode    `tstype:",extends,required"`
	Columns     []ColumnElement `json:"columns"`
	Content     string          `json:"content,omitempty"`     // Prosa suelta dentro del grid pero fuera de cualquier columna
	ContentHTML string          `json:"contentHTML,omitempty"` // ver TextElement.ContentHTML
	// LangRuns cubre solo Content (la prosa suelta del grid) — un span de
	// idioma dentro de Columns aparece en el LangRuns del elemento anidado
	// correspondiente, poblado por recursión sobre esa columna, no aquí.
	// Ver TextElement.LangRuns.
	LangRuns []LangRun `json:"langRuns,omitempty"`
	// DiscardedLangRuns cubre solo Content, mismo alcance que LangRuns arriba.
	// Ver TextElement.DiscardedLangRuns.
	DiscardedLangRuns []LangRun `json:"discardedLangRuns,omitempty"`
}

func (g GridElement) element() {}

// NewGridElement crea un nuevo elemento grid
func NewGridElement(pos diagnostics.Position) *GridElement {
	return &GridElement{
		BaseNode: NewBaseNode(NodeTypeGrid, pos),
		Columns:  make([]ColumnElement, 0),
	}
}

// ColumnElement representa una columna dentro de un grid
type ColumnElement struct {
	BaseNode    `tstype:",extends,required"`
	Content     string `json:"content"`
	ContentHTML string `json:"contentHTML,omitempty"` // ver TextElement.ContentHTML
	// LangRuns cubre solo Content (la prosa suelta de la columna) — un span
	// de idioma dentro de Elements aparece en el LangRuns del elemento
	// anidado correspondiente, poblado por recursión, no aquí. Ver
	// TextElement.LangRuns.
	LangRuns []LangRun `json:"langRuns,omitempty"`
	// DiscardedLangRuns cubre solo Content, mismo alcance que LangRuns arriba.
	// Ver TextElement.DiscardedLangRuns.
	DiscardedLangRuns []LangRun `json:"discardedLangRuns,omitempty"`
	Elements          []Element `json:"elements,omitempty"`
}

func (c ColumnElement) element() {}

// NewColumnElement crea un nuevo elemento column
func NewColumnElement(pos diagnostics.Position, content string) *ColumnElement {
	return &ColumnElement{
		BaseNode: NewBaseNode(NodeTypeColumn, pos),
		Content:  content,
		Elements: make([]Element, 0),
	}
}

// MathElement representa una ecuación/fórmula en bloque (issue #239,
// decisión B). Content es LaTeX crudo — motor de render: MathJax con salida
// SVG (renderer/cdn_tags.go), elegido sobre KaTeX porque su SVG es
// autocontenido (sin web-fonts) y no requiere tocar el CSP de
// renderer/csp.go. Deliberadamente SIN ContentHTML: igual que
// MermaidElement.Content, es fuente de fórmula, no prosa — el cliente
// (MathJax) la renderiza, PopulateInlineHTML no puede. Label/Number: mismo
// mecanismo de referencia cruzada que ImageElement/TableElement (ver esos
// campos), con su propio contador independiente (una ecuación y una figura
// pueden compartir número sin colisionar).
type MathElement struct {
	BaseNode    `tstype:",extends,required"`
	Content     string `json:"content"` // LaTeX crudo
	Label       string `json:"label,omitempty"`
	Number      int    `json:"number,omitempty"`
	Caption     string `json:"caption,omitempty"`
	CaptionHTML string `json:"captionHTML,omitempty"` // Caption con {{variables}} sustituidas y escapadas (sin markdown)
}

func (m MathElement) element() {}

// NewMathElement crea un nuevo elemento de ecuación
func NewMathElement(pos diagnostics.Position, content string) *MathElement {
	return &MathElement{
		BaseNode: NewBaseNode(NodeTypeMath, pos),
		Content:  content,
	}
}
