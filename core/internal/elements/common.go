// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
)

// ElementType representa los tipos de elementos que pueden ser parseados
type ElementType int

const (
	ElementTypeText ElementType = iota
	ElementTypePoints
	ElementTypeCode
	ElementTypeImage
	ElementTypeTable
	ElementTypeSpecialBlock
	ElementTypeMermaid
	ElementTypeChart
	ElementTypeMap
	ElementTypeDirective
	ElementTypeUnknown
)

// ParseContext proporciona contexto para el parsing de elementos
type ParseContext struct {
	Mode        string // "strict" or "flex"
	CurrentLine int
	Logger      util.Logger // Logger interface for structured logging
	Lines       []string
}

// ParseResult encapsula el resultado del parsing de un elemento
type ParseResult struct {
	Element       ast.Element
	ConsumedLines int
	Error         error

	// Diagnostics permite a un ElementParser reportar problemas de contenido
	// (JSON embebido malformado, etc.) sin abortar el build: a diferencia de
	// Error (que los parsers de nivel superior siempre envuelven como
	// severidad Error), estos diagnósticos conservan la severidad que el
	// ElementParser les asigne (típicamente Warning).
	Diagnostics []diagnostics.Diagnostic
}

// ElementParser define la interfaz común para parsers de elementos
type ElementParser interface {
	// CanParse determina si este parser puede manejar la línea dada
	CanParse(line string, mode string) bool

	// Parse parsea el elemento comenzando desde las líneas dadas
	Parse(ctx *ParseContext, startIndex int) *ParseResult
}

// Registry mantiene todos los parsers de elementos registrados
type Registry struct {
	parsers []ElementParser
}

// NewRegistry crea un nuevo registro de parsers
func NewRegistry() *Registry {
	return &Registry{
		parsers: make([]ElementParser, 0),
	}
}

// Register añade un parser al registro
func (r *Registry) Register(parser ElementParser) {
	r.parsers = append(r.parsers, parser)
}

// Parse intenta parsear un elemento usando todos los parsers registrados
func (r *Registry) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	line := ctx.Lines[startIndex]

	// Intentar con cada parser registrado
	for _, parser := range r.parsers {
		if parser.CanParse(line, ctx.Mode) {
			return parser.Parse(ctx, startIndex)
		}
	}

	// Si ningún parser especializado puede manejarlo, crear elemento de texto por defecto
	return &ParseResult{
		Element:       nil,
		ConsumedLines: 0,
		Error:         nil,
	}
}

// GetDefaultRegistry retorna un registro con todos los parsers estándar
func GetDefaultRegistry() *Registry {
	registry := NewRegistry()

	// Registrar parsers en orden de prioridad (más específicos primero)
	registry.Register(&MathParser{}) // Ecuaciones LaTeX (issue #239-B) — prefijo <<math>>/$$ inequívoco
	registry.Register(&MermaidParser{})
	registry.Register(&PlantUMLParser{}) // PlantUML diagrams (UML-focused)
	registry.Register(&ChartParser{})
	registry.Register(&MediaParser{}) // Audio/video embebido (issue #21) — mismo estilo de marcador que ChartParser
	registry.Register(&MapParser{})
	registry.Register(&CodeGroupParser{})
	registry.Register(&CodeParser{})
	registry.Register(&GridParser{}) // Grid layout parser debe ir ANTES que SpecialBlockParser
	registry.Register(&SpecialBlockParser{})
	registry.Register(&DirectiveParser{})
	registry.Register(&ImageParser{})
	registry.Register(&TableParser{})
	registry.Register(&QuoteParser{})     // Citas en bloque
	registry.Register(&ChecklistParser{}) // Listas de tareas con checkboxes
	registry.Register(&PointsParser{})
	registry.Register(&TextParser{}) // TextParser debe ir al final como fallback

	return registry
}

// IsJustASeparator determina si una línea de "cierre" (p. ej. ":::") en el
// índice i es en realidad solo un separador entre sub-bloques repetidos —
// si el próximo contenido no-trivial es un nuevo sub-bloque que empieza con
// subBlockPrefix — en vez del cierre real del bloque padre. Necesario
// porque el marcador de cierre de un sub-bloque (columna, tab, step, etc.)
// y el del bloque padre comparten la misma sintaxis a secas (":::") — sin
// este lookahead, un parser confundiría el cierre de un sub-bloque
// intermedio con el cierre del bloque entero, perdiendo los sub-bloques
// siguientes (issue #57 — extraído del bug de doble-avance encontrado en
// GridParser.Parse, issue #9: sin línea en blanco entre el ":::" de cierre
// de una columna y el "::: column" siguiente, el parser perdía columnas
// completas).
//
// El lookahead se detiene (retorna false) al toparse con closingMarker de
// nuevo, o con cualquier línea que empiece con alguno de blockTerminators
// (p. ej. "SLIDE ") — cualquiera de los dos indica que no queda otro
// sub-bloque antes del fin real del bloque padre.
func IsJustASeparator(lines []string, i int, subBlockPrefix, closingMarker string, blockTerminators ...string) bool {
	for j := i + 1; j < len(lines); j++ {
		next := strings.TrimSpace(lines[j])
		if strings.HasPrefix(next, subBlockPrefix) {
			return true
		}
		if next == closingMarker {
			return false
		}
		for _, term := range blockTerminators {
			if strings.HasPrefix(next, term) {
				return false
			}
		}
	}
	return false
}

// strictTopLevelKeywords son las palabras clave que abren un bloque strict de
// nivel superior: "SLIDE" en slidelang, "SECTION" en doclang. Ver
// IsStrictBlockBoundary.
var strictTopLevelKeywords = []string{"SLIDE", "SECTION"}

// IsStrictBlockBoundary reporta si rawLine (SIN trim) abre un bloque strict de
// nivel superior — "SLIDE " (slidelang) o "SECTION " (doclang), en columna 0,
// sin sangría, con frontera de palabra tras el keyword ("SLIDEfoo"/
// "SECTIONfoo" no cuentan — mismo motivo que startsSectionKeyword en
// parser/document_strict.go, issue #45: un HasPrefix pelado los aceptaría
// pese a no ser el keyword real, y el caller los re-despacharía sin fin).
//
// Se chequea la línea cruda a propósito: es la sangría, no las palabras, lo
// que distingue un límite real de contenido indentado que casualmente
// empieza con esas palabras (una columna de grid, una fila de datos de
// chart...). Cualquier parser strict de bloque delimitado que no tenga un
// marcador de cierre obligatorio en cada línea (chart, math, plantuml)
// necesita este chequeo para no tragarse el bloque siguiente cuando el
// autor omite el cierre — ver isSlideBoundary en grid.go e
// isChartContentBoundary en chart.go para los dos usos actuales; que
// deleguen aquí en vez de reimplementar el criterio si cambia (issue #107).
func IsStrictBlockBoundary(rawLine string) bool {
	for _, kw := range strictTopLevelKeywords {
		if !strings.HasPrefix(rawLine, kw) {
			continue
		}
		rest := rawLine[len(kw):]
		if rest == "" || rest[0] == ' ' || rest[0] == '\t' {
			return true
		}
	}
	return false
}

// CalculateIndentLevel calcula el nivel de indentación de una línea
// Cuenta espacios como 1 y tabs como 4 espacios
func CalculateIndentLevel(line string) int {
	indentLevel := 0
loop:
	for _, char := range line {
		switch char {
		case ' ':
			indentLevel++
		case '\t':
			indentLevel += 4 // Count tabs as 4 spaces
		default:
			break loop
		}
	}
	return indentLevel
}

// AutoDetectIndentation estructura que ayuda con la auto-detección de indentación
type AutoDetectIndentation struct {
	ExpectedIndent int // -1 significa no detectado aún
}

// NewAutoDetectIndentation crea una nueva instancia para auto-detección
func NewAutoDetectIndentation() *AutoDetectIndentation {
	return &AutoDetectIndentation{
		ExpectedIndent: -1,
	}
}

// ShouldProcessLine determina si una línea debería ser procesada como parte del bloque indentado
// Retorna true si la línea debería procesarse, false si debería terminar el bloque
func (a *AutoDetectIndentation) ShouldProcessLine(line string, verbose bool, lineNumber int, parserName string) bool {
	currentIndent := CalculateIndentLevel(line)
	trimmedLine := strings.TrimSpace(line)

	// Skip empty lines
	if trimmedLine == "" {
		return true
	}
	// Auto-detect expected indentation from first non-empty line
	if a.ExpectedIndent == -1 && currentIndent > 0 {
		a.ExpectedIndent = currentIndent
	}

	// Check if this line should be part of the block
	if a.ExpectedIndent > 0 && currentIndent < a.ExpectedIndent && trimmedLine != "" {
		return false
	}
	// If we haven't detected indentation yet and line has no indentation, break
	if a.ExpectedIndent == -1 && currentIndent == 0 && trimmedLine != "" {
		return false
	}

	return true
}

// IsNewElement verifica si una línea inicia un nuevo elemento
// Esta función centraliza la lógica de detección de elementos para evitar duplicación
// entre diferentes parsers.
//
// Parámetros:
//   - line: La línea de texto a verificar
//   - mode: El modo de parsing ("strict" o "flex")
//
// Retorna:
//   - true si la línea inicia un nuevo elemento
//   - false si la línea es contenido de un elemento existente
func IsNewElement(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	if mode == "strict" {
		// Marcadores simbólicos que inician elementos en modo estricto: @
		// directiva, ::: special block/grid/code-group, << diagrama/chart/
		// map/math, | tabla Markdown. Antes solo se reconocían los keywords
		// en mayúsculas de abajo — un CHECKLIST o QUOTE inmediatamente
		// seguido (sin línea en blanco) por, p. ej., un `<<math>>` o
		// `::: info` se tragaba ese elemento hermano como si fuera
		// contenido/continuación propio, en vez de cortar el loop (misma
		// clase de bug que el fix de IMAGE: "strict IMAGE deja de tragarse
		// elementos hermanos"). Espeja startsStrictElement en
		// parser/strict.go y strictNewElementKeywords en
		// formatter/strict.go — mantenerlos en sync si esta lista cambia.
		if len(trimmed) > 0 {
			switch trimmed[0] {
			case '@', '|':
				return true
			case ':':
				if strings.HasPrefix(trimmed, ":::") {
					return true
				}
			case '<':
				if strings.HasPrefix(trimmed, "<<") {
					return true
				}
			}
		}

		// Keywords que inician elementos en modo estricto
		keywords := []string{
			"TEXT", "POINTS", "CODE", "IMAGE", "TABLE",
			"QUOTE", "CHECKLIST", "MERMAID", "CHART", "MAP",
			"DIRECTIVE", "SPECIAL_BLOCK", "CODE_GROUP", "MATH",
		}
		for _, keyword := range keywords {
			if strings.HasPrefix(trimmed, keyword) {
				return true
			}
		}
	}

	if mode == "flex" {
		// Primero verificar si es un formato [x] pero no un checklist válido
		// Esto debe hacerse ANTES de verificar listas regulares
		if len(trimmed) >= 5 &&
			(strings.HasPrefix(trimmed, "- [") ||
				strings.HasPrefix(trimmed, "* [") ||
				strings.HasPrefix(trimmed, "+ [")) {
			// Verificar que el siguiente carácter después de [ sea seguido inmediatamente por ]
			if trimmed[4] == ']' {
				// Verificar que el carácter dentro de los corchetes sea válido para checklist
				checkChar := trimmed[3]
				if checkChar == ' ' || checkChar == 'x' || checkChar == 'X' {
					return true // Es un checklist válido
				}
				// Si tiene formato [x] pero no es un checklist válido, no es ni checklist ni lista
				return false
			} else {
				// Si tiene formato "- [algo..." pero no es [x], no es ni checklist ni lista
				// Es texto regular que casualmente empieza con "- ["
				return false
			}
		}

		// Elementos que inician con sintaxis específica en modo flex
		if strings.HasPrefix(trimmed, "#") || // Headers
			strings.HasPrefix(trimmed, "- ") || // Lists
			strings.HasPrefix(trimmed, "* ") || // Lists
			strings.HasPrefix(trimmed, "+ ") || // Lists
			strings.HasPrefix(trimmed, "```") || // Code blocks
			strings.HasPrefix(trimmed, "![") || // Images
			strings.HasPrefix(trimmed, "|") || // Tables
			strings.HasPrefix(trimmed, "> ") || // Quotes
			strings.HasPrefix(trimmed, ":::") { // Special blocks
			return true
		}

		// Listas numeradas (1. 2. 3. etc.)
		if len(trimmed) > 2 && trimmed[1] == '.' && trimmed[0] >= '0' && trimmed[0] <= '9' {
			return true
		}
	}

	return false
}

// splitTopLevelCommas parte s por las comas que están FUERA de comillas
// dobles y devuelve los items EN BRUTO — sin recortar espacios ni quitar las
// comillas — junto con ok=false si el conteo de comillas quedó impar.
//
// Devolver el item en bruto es lo que permite compartirlo entre llamadores con
// necesidades distintas: elements.TableParser desenvuelve la celda (una celda
// es siempre texto), mientras ChartParser necesita ver las comillas intactas
// para decidir si el item es una etiqueta o un número. Cada llamador se queda
// también con su propio manejo de items vacíos (una fila de chart los ignora;
// una fila de tabla conserva la celda vacía de una coma final).
//
// El ok=false es una red de seguridad, no una validación: con un conteo impar
// de comillas la entrada está malformada y el llamador debe caer a su
// comportamiento previo (el split ingenuo por comas) en vez de inventar una
// interpretación. Ver splitInlineArray (internal/elements/table.go) para la
// política completa —incluido por qué el backslash NO es escape acá.
//
// No absorbe a parseCSVLine (internal/elements/map.go), que resuelve el mismo
// problema para los marcadores de <<map>>: aquel CONSERVA las comillas dentro
// del campo y descarta el último campo si quedó vacío. Es otro contrato, con
// sus propias fixtures; unificarlos sería un cambio de comportamiento en
// mapas disfrazado de refactor.
func splitTopLevelCommas(s string) ([]string, bool) {
	items := []string{}
	start := 0
	inQuotes := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				items = append(items, s[start:i])
				start = i + 1
			}
		}
	}
	if inQuotes {
		return nil, false
	}
	return append(items, s[start:]), true
}
