// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/ast"
)

// ChecklistParser maneja elementos de listas de tareas/checklists
type ChecklistParser struct{}

// CanParse determina si una línea es el inicio de una lista de tareas
func (p *ChecklistParser) CanParse(line string, mode string) bool {
	trimmed := strings.TrimSpace(line)

	// Strict mode: CHECKLIST keyword
	if mode == "strict" && HasStrictKeyword(trimmed, "CHECKLIST") {
		return true
	}

	// Both modes: Markdown checklist syntax
	return p.isChecklistItem(trimmed)
}

// Parse parsea una lista de tareas desde las líneas proporcionadas
func (p *ChecklistParser) Parse(ctx *ParseContext, startIndex int) *ParseResult {
	if startIndex >= len(ctx.Lines) {
		return &ParseResult{
			Element:       nil,
			ConsumedLines: 0,
			Error:         nil,
		}
	}

	pos := ctx.Position(startIndex)
	element := ast.NewChecklistElement(pos)
	consumed := 0
	line := strings.TrimSpace(ctx.Lines[startIndex])

	if ctx.Mode == "strict" && HasStrictKeyword(line, "CHECKLIST") {
		// Skip CHECKLIST line
		consumed++
		startIndex++

		// Process indented checklist items
		consumed += p.parseStrictChecklist(ctx, startIndex, element)
	} else {
		// Markdown-style parsing
		consumed = p.parseMarkdownChecklist(ctx, startIndex, element)
	}

	// Set end position
	if consumed > 0 {
		// índice de la última línea consumida (startIndex+consumed-1); misma
		// aritmética que antes del offset (#245).
		element.EndPosition = ctx.Position(startIndex + consumed - 1)
	}

	return &ParseResult{
		Element:       element,
		ConsumedLines: consumed,
		Error:         nil,
	}
}

// parseStrictChecklist parsea una lista de tareas en modo estricto
func (p *ChecklistParser) parseStrictChecklist(ctx *ParseContext, startIndex int, element *ast.ChecklistElement) int {
	lines := ctx.Lines
	consumed := 0
	expectedIndent := -1
	nest := checklistNest{strict: true}

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Calculate current line indentation
		currentIndent := CalculateIndentLevel(line)

		// Skip empty lines
		if trimmed == "" {
			consumed++
			continue
		}

		// Auto-detect expected indentation from first non-empty line
		if expectedIndent == -1 && currentIndent > 0 {
			expectedIndent = currentIndent
		}

		// Check if this line should be part of the checklist block
		if expectedIndent > 0 && currentIndent < expectedIndent && trimmed != "" {
			break
		}

		// Check if line starts a new element
		if IsNewElement(line, "strict") {
			break
		}

		// Parse checklist item
		if p.isStrictChecklistItem(trimmed) {
			content, checked := p.parseStrictChecklistContent(trimmed)
			if content != "" {
				nest.add(element, ast.NewChecklistItem(ctx.Position(i), content, checked), currentIndent, expectedIndent, trimmed)
			}
		} else if last := nest.last(); currentIndent >= expectedIndent && last != nil {
			// Continuation line: pertenece al último item abierto, el más
			// profundo, que es el que la línea sigue.
			last.Content += " " + trimmed
		}

		consumed++
	}

	return consumed
}

// parseMarkdownChecklist parsea una lista de tareas en formato Markdown
func (p *ChecklistParser) parseMarkdownChecklist(ctx *ParseContext, startIndex int, element *ast.ChecklistElement) int {
	lines := ctx.Lines
	consumed := 0
	baseIndent := -1
	var nest checklistNest

	for i := startIndex; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Stop at empty lines
		if trimmed == "" {
			// Allow one empty line within checklist
			if i+1 < len(lines) && p.isChecklistItem(strings.TrimSpace(lines[i+1])) {
				consumed++
				continue
			}
			break
		}

		// Check if this is a checklist item
		if !p.isChecklistItem(trimmed) {
			break
		}

		// Sangría en columnas, con tabuladores a paradas de 4 como CommonMark.
		indent := IndentColumns(line)
		if baseIndent == -1 {
			baseIndent = indent
		}

		// Extract content and checked status
		content, checked := p.parseChecklistContent(trimmed)
		if content != "" {
			nest.add(element, ast.NewChecklistItem(ctx.Position(i), content, checked), indent, baseIndent, trimmed)
		}

		consumed++
	}

	return consumed
}

// checklistNest arma el árbol de una lista de tareas con la misma regla que
// legacyNest (points.go): una tarea es sub-tarea de la de arriba solo si su
// marcador empieza en la columna de contenido de esa tarea o más adentro (la
// que sigue a "- "); en strict, si está más sangrada que ella. Antes solo había
// dos niveles: todo lo que pasaba del base se colgaba del último item base.
// SubItems ya era recursivo en el AST y en el esquema, así que el JSON no
// cambia de forma.
type checklistNest struct {
	stack []checklistLevel
	// strict: cualquier tarea más sangrada que la de arriba es su sub-tarea,
	// como en legacyNest.
	strict bool
}

type checklistLevel struct {
	content int
	item    *ast.ChecklistItem
}

func (n *checklistNest) add(element *ast.ChecklistElement, item *ast.ChecklistItem, indent, baseIndent int, trimmed string) {
	if indent < baseIndent {
		return
	}
	// "- [x] a": el contenido de la tarea empieza después de "- ". En la
	// forma strict "[x] a" no hay viñeta, y basta una columna más.
	width := 1
	if !n.strict && trimmed != "" && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') {
		width = 2
	}
	level := checklistLevel{content: indent + width}
	for len(n.stack) > 0 && indent < n.stack[len(n.stack)-1].content {
		n.stack = n.stack[:len(n.stack)-1]
	}
	if len(n.stack) == 0 {
		element.Items = append(element.Items, *item)
		level.item = &element.Items[len(element.Items)-1]
		n.stack = append(n.stack, level)
		return
	}
	parent := n.stack[len(n.stack)-1].item
	parent.SubItems = append(parent.SubItems, *item)
	level.item = &parent.SubItems[len(parent.SubItems)-1]
	n.stack = append(n.stack, level)
}

func (n *checklistNest) last() *ast.ChecklistItem {
	if len(n.stack) == 0 {
		return nil
	}
	return n.stack[len(n.stack)-1].item
}

// isChecklistItem verifica si una línea es un item de checklist
func (p *ChecklistParser) isChecklistItem(line string) bool {
	// Markdown checklist format: - [ ] or - [x] or - [X]
	re := regexp.MustCompile(`^[-*+]\s*\[([xX\s])\]\s*(.*)`)
	return re.MatchString(line)
}

// parseChecklistContent extrae el contenido y estado de un item de checklist
func (p *ChecklistParser) parseChecklistContent(line string) (string, bool) {
	re := regexp.MustCompile(`^[-*+]\s*\[([xX\s])\]\s*(.*)`)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 3 {
		return "", false
	}

	checked := strings.ToLower(matches[1]) == "x"
	content := strings.TrimSpace(matches[2])

	return content, checked
}

// strictChecklistItemRe acepta tanto "[x] item" (histórico) como
// "- [x] item"/"* [x] item"/"+ [x] item" (issue del audit 2026-09-11, F6):
// CHECKLIST en modo strict exigía la forma pelada, inconsistente con POINTS
// (que sí acepta bullet en strict, points.go) y con CHECKLIST en modo flex
// (isChecklistItem, arriba) — un autor que escribía "- [x] item" bajo
// CHECKLIST en strict veía la lista renderizarse vacía, sin ningún
// diagnóstico (gallery/01_strict_mode_basics.slidelang usaba POINTS en su
// lugar para esquivar esto, en vez de CHECKLIST). El bullet es opcional
// (?:...)? para no romper el formato pelado ya existente.
var strictChecklistItemRe = regexp.MustCompile(`^(?:[-*+]\s*)?\[([xX\s])\]\s*(.*)`)

// isStrictChecklistItem verifica si una línea es un item de checklist en modo estricto
func (p *ChecklistParser) isStrictChecklistItem(line string) bool {
	return strictChecklistItemRe.MatchString(line)
}

// parseStrictChecklistContent extrae el contenido y estado de un item de checklist en modo estricto
func (p *ChecklistParser) parseStrictChecklistContent(line string) (string, bool) {
	matches := strictChecklistItemRe.FindStringSubmatch(line)

	if len(matches) < 3 {
		return "", false
	}

	checked := strings.ToLower(matches[1]) == "x"
	content := strings.TrimSpace(matches[2])

	return content, checked
}
