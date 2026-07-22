// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"encoding/json"

	"go.ziradocs.com/core/v2/diagnostics"
)

// HeaderFooterConfig representa la configuración global de headers y footers
type HeaderFooterConfig struct {
	Header         *HeaderConfig                        `json:"header,omitempty"`
	Footer         *FooterConfig                        `json:"footer,omitempty"`
	LayoutDefaults map[string]*LayoutHeaderFooterConfig `json:"layout_defaults,omitempty"`
}

// HeaderConfig configura la apariencia y contenido del header
type HeaderConfig struct {
	Enabled    bool              `json:"enabled"`
	Height     string            `json:"height,omitempty"`     // e.g., "60px", "4rem"
	Background string            `json:"background,omitempty"` // color o gradiente
	Text       *HeaderFooterText `json:"text,omitempty"`
	Logo       *LogoConfig       `json:"logo,omitempty"`
	Border     *BorderConfig     `json:"border,omitempty"`
}

// FooterConfig configura la apariencia y contenido del footer
type FooterConfig struct {
	Enabled     bool               `json:"enabled"`
	Height      string             `json:"height,omitempty"`     // e.g., "40px", "3rem"
	Background  string             `json:"background,omitempty"` // color o gradiente
	Text        *HeaderFooterText  `json:"text,omitempty"`
	PageNumbers *PageNumbersConfig `json:"page_numbers,omitempty"`
	Border      *BorderConfig      `json:"border,omitempty"`
}

// HeaderFooterText define el contenido de texto en headers/footers
type HeaderFooterText struct {
	Left   string `json:"left,omitempty"`
	Center string `json:"center,omitempty"`
	Right  string `json:"right,omitempty"`
}

// PageNumbersConfig configura la numeración de páginas
type PageNumbersConfig struct {
	Enabled              bool   `json:"enabled"`
	Format               string `json:"format,omitempty"`   // e.g., "{{current}} / {{total}}", "Página {{current}}"
	Position             string `json:"position,omitempty"` // "left", "center", "right"
	ExcludeTitleSlides   bool   `json:"exclude_title_slides,omitempty"`
	ExcludeClosingSlides bool   `json:"exclude_closing_slides,omitempty"`
	StartFrom            int    `json:"start_from,omitempty"`
	Style                string `json:"style,omitempty"` // "normal", "caption", "bold"
}

// LogoConfig configura logos en headers
type LogoConfig struct {
	Source   string `json:"source,omitempty"`   // ruta al logo
	Alt      string `json:"alt,omitempty"`      // texto alternativo
	Height   string `json:"height,omitempty"`   // altura del logo
	Position string `json:"position,omitempty"` // "left", "center", "right"
}

// BorderConfig configura bordes en headers/footers
type BorderConfig struct {
	Enabled  bool   `json:"enabled"`
	Color    string `json:"color,omitempty"`
	Width    string `json:"width,omitempty"`    // e.g., "1px", "2px"
	Style    string `json:"style,omitempty"`    // "solid", "dashed", "dotted"
	Position string `json:"position,omitempty"` // "top", "bottom", "both"
}

// LayoutHeaderFooterConfig permite overrides por tipo de layout
type LayoutHeaderFooterConfig struct {
	Header *HeaderConfig `json:"header,omitempty"`
	Footer *FooterConfig `json:"footer,omitempty"`
}

// ContentBlockHeaderFooterOverride permite overrides por bloque de contenido individual
type ContentBlockHeaderFooterOverride struct {
	Header *HeaderConfig `json:"header,omitempty"`
	Footer *FooterConfig `json:"footer,omitempty"`
}

// FrontMatterNode contiene el YAML parseado del FrontMatter
type FrontMatterNode struct {
	BaseNode     `tstype:",extends,required"`
	Mode         string                 `json:"mode"`
	Title        string                 `json:"title,omitempty"`
	Author       string                 `json:"author,omitempty"`
	Date         string                 `json:"date,omitempty"`
	Theme        string                 `json:"theme,omitempty"`
	Variables    map[string]interface{} `json:"variables,omitempty"`
	HeaderFooter *HeaderFooterConfig    `json:"header_footer,omitempty"` // Nueva configuración
	Raw          string                 `json:"-"`                       // YAML crudo
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

	for k, v := range fm.Variables {
		variables[k] = v
	}

	return variables
}

// ContentBlock representa un bloque de contenido (slide en presentaciones, sección en documentos)
type ContentBlock struct {
	BaseNode  `tstype:",extends,required"`
	BlockType string `json:"blockType,omitempty"` // "title", "content", "section", etc.
	Title     string `json:"title,omitempty"`
	TitleHTML string `json:"titleHTML,omitempty"` // Title con {{variables}} sustituidas y escapadas (sin markdown)
	// Propiedades específicas para bloques tipo "title" (usado en presentaciones)
	Heading      string    `json:"heading,omitempty"`
	HeadingHTML  string    `json:"headingHTML,omitempty"` // Heading con {{variables}} sustituidas y escapadas (sin markdown)
	Subtitle     string    `json:"subtitle,omitempty"`
	SubtitleHTML string    `json:"subtitleHTML,omitempty"` // Subtitle con {{variables}} sustituidas y escapadas (sin markdown)
	Logo         string    `json:"logo,omitempty"`
	Elements     []Element `json:"elements"`
	// Configuración específica de header/footer para este bloque
	HeaderFooterOverride *ContentBlockHeaderFooterOverride `json:"header_footer_override,omitempty"`
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
	BaseNode    `tstype:",extends,required"`
	Content     string      `json:"content"`
	ContentHTML string      `json:"contentHTML,omitempty"` // ver TextElement.ContentHTML
	SubPoints   []PointItem `json:"subPoints,omitempty"`
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

// TableElement representa una tabla con datos
type TableElement struct {
	BaseNode    `tstype:",extends,required"`
	Headers     []string   `json:"headers"`
	HeadersHTML []string   `json:"headersHTML,omitempty"` // Headers ya renderizados a HTML inline (ver TextElement.ContentHTML)
	Rows        [][]string `json:"rows"`
	RowsHTML    [][]string `json:"rowsHTML,omitempty"` // Rows ya renderizadas a HTML inline (ver TextElement.ContentHTML)
	Caption     string     `json:"caption,omitempty"`
	CaptionHTML string     `json:"captionHTML,omitempty"` // Caption con {{variables}} sustituidas y escapadas (sin markdown)
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
	Icon        string `json:"icon,omitempty"`
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
	TitleHTML   string `json:"titleHTML,omitempty"` // Title con {{variables}} sustituidas y escapadas (sin markdown); Content es fuente de diagrama, no lleva *HTML (ver docs/architecture/json-ast-contract.md)
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
	Author      string `json:"author,omitempty"`      // Para citas con autor
	AuthorHTML  string `json:"authorHTML,omitempty"`  // Author con {{variables}} sustituidas y escapadas (sin markdown)
	Source      string `json:"source,omitempty"`      // Para citas con fuente
	SourceHTML  string `json:"sourceHTML,omitempty"`  // Source con {{variables}} sustituidas y escapadas (sin markdown)
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
	BaseNode    `tstype:",extends,required"`
	Content     string          `json:"content"`
	ContentHTML string          `json:"contentHTML,omitempty"` // ver TextElement.ContentHTML
	Checked     bool            `json:"checked"`
	SubItems    []ChecklistItem `json:"subItems,omitempty"`
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
	Content     string    `json:"content"`
	ContentHTML string    `json:"contentHTML,omitempty"` // ver TextElement.ContentHTML
	Elements    []Element `json:"elements,omitempty"`
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
