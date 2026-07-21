// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package data

import htmltemplate "html/template"

// PresentationData contiene todos los datos para el template HTML
type PresentationData struct {
	Title         string
	Author        string
	Date          string
	ContentBlocks []SlideData `json:"content_blocks"` // Bloques de contenido de la presentación
	HasTitle      bool
	HeaderFooter  *HeaderFooterData `json:"header_footer,omitempty"`

	// NUEVOS CAMPOS PARA VISUALIZADOR AVANZADO
	Version           string                `json:"version"` // "2.0.0"
	TotalSlides       int                   `json:"total_slides"`
	EstimatedDuration int                   `json:"estimated_duration"` // en segundos
	Features          *PresentationFeatures `json:"features"`
	RequiredLibraries []string              `json:"required_libraries"`
	Theme             string                `json:"theme"`    // Tema efectivo usado en la presentación
	Charts            []ChartMetadata       `json:"charts"`   // Charts metadata for JavaScript rendering
	Diagrams          []MermaidMetadata     `json:"diagrams"` // Mermaid diagrams metadata for JavaScript rendering
	Maps              []MapMetadata         `json:"maps"`     // Maps metadata for JavaScript rendering
}

// PresentationFeatures contiene flags de características detectadas
type PresentationFeatures struct {
	HasMermaid bool `json:"has_mermaid"`
	HasCharts  bool `json:"has_charts"`
	HasMaps    bool `json:"has_maps"`
	HasCode    bool `json:"has_code"`
	HasQuotes  bool `json:"has_quotes"`
	HasNotes   bool `json:"has_notes"` // true si hay notas del presentador
}

// HeaderFooterData contiene la configuración procesada para headers y footers
type HeaderFooterData struct {
	GlobalHeader    *HeaderData                        `json:"global_header,omitempty"`
	GlobalFooter    *FooterData                        `json:"global_footer,omitempty"`
	LayoutDefaults  map[string]*LayoutHeaderFooterData `json:"layout_defaults,omitempty"`
	TotalSlides     int                                `json:"total_slides"`
	CountableSlides int                                `json:"countable_slides"` // Slides que cuentan para numeración
}

// HeaderData representa datos procesados del header
type HeaderData struct {
	Enabled    bool                  `json:"enabled"`
	Height     string                `json:"height,omitempty"`
	Background string                `json:"background,omitempty"`
	Text       *HeaderFooterTextData `json:"text,omitempty"`
	Logo       *LogoData             `json:"logo,omitempty"`
	Border     *BorderData           `json:"border,omitempty"`
}

// FooterData representa datos procesados del footer
type FooterData struct {
	Enabled     bool                  `json:"enabled"`
	Height      string                `json:"height,omitempty"`
	Background  string                `json:"background,omitempty"`
	Text        *HeaderFooterTextData `json:"text,omitempty"`
	PageNumbers *PageNumbersData      `json:"page_numbers,omitempty"`
	Border      *BorderData           `json:"border,omitempty"`
}

// HeaderFooterTextData contiene texto procesado con variables
type HeaderFooterTextData struct {
	Left   string `json:"left,omitempty"`
	Center string `json:"center,omitempty"`
	Right  string `json:"right,omitempty"`
}

// PageNumbersData contiene configuración de numeración procesada
type PageNumbersData struct {
	Enabled              bool   `json:"enabled"`
	Format               string `json:"format,omitempty"`
	Position             string `json:"position,omitempty"`
	ExcludeTitleSlides   bool   `json:"exclude_title_slides,omitempty"`
	ExcludeClosingSlides bool   `json:"exclude_closing_slides,omitempty"`
	StartFrom            int    `json:"start_from,omitempty"`
	Style                string `json:"style,omitempty"`
}

// LogoData contiene configuración de logo procesada
type LogoData struct {
	Source   string `json:"source,omitempty"`
	Alt      string `json:"alt,omitempty"`
	Height   string `json:"height,omitempty"`
	Position string `json:"position,omitempty"`
}

// BorderData contiene configuración de borde procesada
type BorderData struct {
	Enabled  bool   `json:"enabled"`
	Color    string `json:"color,omitempty"`
	Width    string `json:"width,omitempty"`
	Style    string `json:"style,omitempty"`
	Position string `json:"position,omitempty"`
}

// LayoutHeaderFooterData contiene overrides por layout
type LayoutHeaderFooterData struct {
	Header *HeaderData `json:"header,omitempty"`
	Footer *FooterData `json:"footer,omitempty"`
}

// SlideData representa una slide para el template
type SlideData struct {
	Type  string
	Title string
	// Propiedades específicas para slides tipo "title"
	Heading   string
	Subtitle  string
	Logo      string
	Elements  []ElementData
	IsTitle   bool
	IsContent bool
	// Numeración específica del slide
	SlideNumber    int  `json:"slide_number"`     // Número absoluto (1, 2, 3...)
	DisplayNumber  int  `json:"display_number"`   // Número para mostrar (puede empezar en start_from)
	ShowPageNumber bool `json:"show_page_number"` // Si debe mostrar numeración en este slide
	// Header/Footer específico del slide
	HeaderFooterOverride *SlideHeaderFooterData `json:"header_footer_override,omitempty"`

	// NUEVOS CAMPOS PARA VISUALIZADOR AVANZADO
	Duration            int      `json:"duration"`             // Duración estimada en segundos
	Transition          string   `json:"transition"`           // "fade", "slide", "zoom"
	HasInteractive      bool     `json:"has_interactive"`      // true si tiene elementos interactivos
	InteractiveElements []string `json:"interactive_elements"` // ["mermaid", "chart", "map"]
	Notes               []string `json:"notes"`                // Presenter notes for this slide
}

// SlideHeaderFooterData contiene overrides específicos del slide
type SlideHeaderFooterData struct {
	Header *HeaderData `json:"header,omitempty"`
	Footer *FooterData `json:"footer,omitempty"`
}

// ElementData representa un elemento para el template
type ElementData struct {
	Type     string
	Content  string
	Language string
	Source   string
	Alt      string
	Caption  string
	Items    []PointItemData
	ListType string // "ordered" para <ol>, "unordered" para <ul>
	// Checklist data
	ChecklistItems []ChecklistItemData
	// Grid data
	Columns []ColumnData
	// Table data
	Headers []string
	Rows    [][]string
	// Special block data
	BlockType string
	Title     string
	Icon      string
	// Code group data
	CodeBlocks []CodeBlockData
	// Mermaid data
	DiagramType string // Chart data
	ChartType   string
	SeriesTypes []string
	ChartData   [][]interface{}
	Series      []string
	Labels      []string
	Options     map[string]interface{}
	IsJSONMode  bool // Indica si usa JSON directo
	// Map data
	MapType    string
	Markers    []MapMarkerData
	Heatmap    bool
	Zoom       int
	MapOptions map[string]interface{}
	// Quote data
	Author string // Autor de la cita
	// Image context data
	Context string `json:"context,omitempty"` // hero, gallery, content, standalone
	// Directive data
	DirectiveName   string                 `json:"directive_name,omitempty"`
	DirectiveParams map[string]interface{} `json:"directive_params,omitempty"`
	CSSClasses      []string               `json:"css_classes,omitempty"`

	// NUEVOS CAMPOS PARA VISUALIZADOR AVANZADO
	ElementID  string `json:"element_id,omitempty"` // ID único del elemento
	SlideIndex int    `json:"slide_index"`          // Índice del slide contenedor

	// PreRenderedHTML contiene el HTML pre-renderizado de un elemento
	// interactivo (mermaid/chart/map) cuando se construye en modo offline
	// (issue #92): un <img src="assets/..."> (offline-assets) o un SVG/data-URI
	// inline (offline-inline), producido por renderer.RenderElementToHTML. Vacío
	// en modo browser (el template cae al placeholder client-side). Es
	// template.HTML porque el contenido ya viene escapado por el core; json:"-"
	// porque solo alimenta el template HTML, no la serialización JSON del AST.
	PreRenderedHTML htmltemplate.HTML `json:"-"`
}

// CodeBlockData representa un bloque de código en un grupo
type CodeBlockData struct {
	Language string
	Label    string
	Content  string
}

// MapMarkerData representa un marcador en un mapa
type MapMarkerData struct {
	Lat     float64
	Lng     float64
	Label   string
	Value   float64
	Color   string
	Size    string
	Details string
}

// PointItemData representa un item de lista
type PointItemData struct {
	Content   string
	SubPoints []PointItemData
}

// ChecklistItemData representa un item de checklist
type ChecklistItemData struct {
	Content  string
	Checked  bool
	SubItems []ChecklistItemData
}

// ChartMetadata representa metadatos de charts para renderizado JavaScript
type ChartMetadata struct {
	ID     string                 `json:"id"`     // ID único del elemento canvas
	Type   string                 `json:"type"`   // Tipo Chart.js
	Config map[string]interface{} `json:"config"` // Configuración completa Chart.js
}

// MermaidMetadata representa metadatos de diagramas Mermaid para renderizado JavaScript
type MermaidMetadata struct {
	ID          string `json:"id"`          // ID único del elemento div
	DiagramType string `json:"diagramType"` // Tipo de diagrama (graph, sequence, class, etc.)
	Content     string `json:"content"`     // Definición del diagrama en sintaxis Mermaid
	Title       string `json:"title"`       // Título opcional del diagrama
}

// MapMetadata representa metadatos de mapas para renderizado JavaScript
type MapMetadata struct {
	ID      string                 `json:"id"`      // ID único del elemento div
	MapType string                 `json:"mapType"` // Tipo de mapa (world, country, region)
	Markers []MapMarkerData        `json:"markers"` // Marcadores del mapa
	Options map[string]interface{} `json:"options"` // Configuración del mapa
	Title   string                 `json:"title"`   // Título opcional del mapa
	Heatmap bool                   `json:"heatmap,omitempty"`
	Zoom    int                    `json:"zoom,omitempty"`
}

// ColumnData representa una columna en un grid layout
type ColumnData struct {
	Content string `json:"content"`
}
