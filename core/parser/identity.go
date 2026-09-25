// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

var nodeIDDirective = regexp.MustCompile(`^<!-- node-id: ([^[:space:]]+) -->$`)

type pendingNodeID struct {
	id     string
	line   int // original, authored source line
	target int // line after removing annotations
}

// stripNodeIDDirectives removes only standalone annotations outside literal
// regions and frontmatter. The ordinary parser then sees the exact source it
// would see if the author had never added annotations. lineMap translates
// parser positions back to the authored source.
func stripNodeIDDirectives(source string) (string, []pendingNodeID, []int, []diagnostics.Diagnostic) {
	lines := strings.Split(source, "\n")
	var pending []pendingNodeID
	var diags []diagnostics.Diagnostic
	removed := make(map[int]bool)
	frontmatterEnd := -1
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		for i := 1; i < len(lines); i++ {
			if strings.TrimSpace(lines[i]) == "---" {
				frontmatterEnd = i
				break
			}
		}
		if frontmatterEnd < 0 {
			frontmatterEnd = len(lines) - 1
		}
	}
	literal := ""
	codeIndent := -1
	groupFence := false
	fenceClose := "```"
	for i, line := range lines {
		if i <= frontmatterEnd {
			continue
		}
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if literal == "code" && trimmed != "" {
			if codeIndent < 0 {
				if indent == 0 {
					literal = ""
				} else {
					codeIndent = indent
				}
			} else if indent < codeIndent {
				literal = ""
			}
		}
		if literal != "" {
			switch literal {
			case "fence":
				if trimmed == fenceClose {
					literal = ""
				}
			case "diagram":
				if trimmed == "<<end>>" {
					literal = ""
				}
			case "math":
				if trimmed == "$$" {
					literal = ""
				}
			case "plantuml":
				if trimmed == "@enduml" {
					literal = ""
				}
			case "group":
				if strings.HasPrefix(trimmed, "```") {
					groupFence = !groupFence
				}
				if trimmed == ":::" && !groupFence {
					literal = ""
				}
			}
			if literal != "" || trimmed == fenceClose || trimmed == "<<end>>" || trimmed == ":::" || trimmed == "$$" || trimmed == "@enduml" {
				continue
			}
		}
		if strings.HasPrefix(trimmed, "```") {
			literal = "fence"
			if strings.HasPrefix(trimmed, "````") {
				fenceClose = "````"
			} else {
				fenceClose = "```"
			}
			continue
		}
		if trimmed == "$$" {
			literal = "math"
			continue
		}
		if strings.HasPrefix(trimmed, "@startuml") {
			literal = "plantuml"
			continue
		}
		if trimmed == "<<mermaid>>" || trimmed == "<<plantuml>>" || trimmed == "<<math>>" || trimmed == "<<map>>" || strings.HasPrefix(trimmed, "<<chart:") {
			literal = "diagram"
			continue
		}
		if strings.HasPrefix(trimmed, ":::code-group") {
			literal = "group"
			groupFence = false
			continue
		}
		if trimmed == "CODE" || strings.HasPrefix(trimmed, "CODE ") {
			literal = "code"
			codeIndent = -1
			continue
		}
		if !strings.HasPrefix(trimmed, "<!-- node-id:") {
			continue
		}
		pos := diagnostics.NewPosition(i+1, indent+1)
		m := nodeIDDirective.FindStringSubmatch(trimmed)
		if m == nil || !ast.ValidNodeID(m[1]) {
			diags = append(diags, diagnostics.NewError("invalid node-id directive: expected <!-- node-id: Name --> (1-128 ASCII letters, digits, '.', '_' or '-', starting with a letter)", pos, "identity"))
			removed[i] = true
			continue
		}
		pending = append(pending, pendingNodeID{m[1], i + 1, 0})
		removed[i] = true
	}
	kept := make([]string, 0, len(lines))
	lineMap := []int{0} // one-based processed line -> authored line
	originalToKept := make(map[int]int)
	for i, line := range lines {
		if removed[i] {
			continue
		}
		kept = append(kept, line)
		lineMap = append(lineMap, i+1)
		originalToKept[i+1] = len(kept)
	}
	for i := range pending {
		target := pending[i].line
		for target < len(lines) && (removed[target] || strings.TrimSpace(lines[target]) == "") {
			target++
		}
		pending[i].target = originalToKept[target+1] // zero for EOF/orphan
	}
	return strings.Join(kept, "\n"), pending, lineMap, diags
}

func bindNodeIDs(doc *ast.AST, pending []pendingNodeID, source string) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	if doc == nil {
		return diags
	}
	byLine := make(map[int][]ast.IdentityNode)
	lines := strings.Split(source, "\n")
	_ = ast.Walk(doc, func(node ast.Node) error {
		if n, ok := node.(ast.IdentityNode); ok && node != doc {
			byLine[node.GetPosition().Line] = append(byLine[node.GetPosition().Line], n)
		}
		return nil
	})
	for _, marker := range pending {
		pos := diagnostics.NewPosition(marker.line, 1)
		matches := byLine[marker.target]
		if len(matches) > 1 && marker.target > 0 && marker.target <= len(lines) {
			// In auto mode a literal "SLIDE ..." can yield both a block
			// and a text node on the same line. The explicit opener names the
			// block; no text or positional similarity is involved.
			line := strings.TrimSpace(lines[marker.target-1])
			if strings.HasPrefix(line, "SLIDE ") || strings.HasPrefix(line, "SECTION ") || strings.HasPrefix(line, "# ") {
				var blocks []ast.IdentityNode
				for _, n := range matches {
					if n.GetType() == ast.NodeTypeContentBlock {
						blocks = append(blocks, n)
					}
				}
				if len(blocks) == 1 {
					matches = blocks
				}
			}
		}
		if len(matches) == 0 {
			diags = append(diags, diagnostics.NewError(fmt.Sprintf("orphan node-id %q: next source line does not start an AST node", marker.id), pos, "identity"))
			continue
		}
		if len(matches) != 1 || matches[0].GetNodeID() != "" {
			diags = append(diags, diagnostics.NewError(fmt.Sprintf("ambiguous node-id %q: next source line has multiple nodes or another ID", marker.id), pos, "identity"))
			continue
		}
		matches[0].SetNodeID(marker.id)
	}
	return diags
}

func restoreNodePositions(doc *ast.AST, lineMap []int) {
	if doc == nil {
		return
	}
	translate := func(pos diagnostics.Position) diagnostics.Position {
		if pos.Line > 0 && pos.Line < len(lineMap) {
			pos.Line = lineMap[pos.Line]
		}
		return pos
	}
	_ = ast.Walk(doc, func(node ast.Node) error {
		if n, ok := node.(interface {
			SetPositions(diagnostics.Position, diagnostics.Position)
		}); ok {
			n.SetPositions(translate(node.GetPosition()), translate(node.GetEndPosition()))
		}
		if table, ok := node.(*ast.TableElement); ok {
			for i := range table.RowPositions {
				table.RowPositions[i] = translate(table.RowPositions[i])
			}
		}
		return nil
	})
	if doc.FrontMatter != nil {
		doc.FrontMatter.SetPositions(translate(doc.FrontMatter.Position), translate(doc.FrontMatter.EndPosition))
	}
}

func restoreDiagnosticPositions(diags []diagnostics.Diagnostic, lineMap []int) {
	for i := range diags {
		line := diags[i].Position.Line
		if line > 0 && line < len(lineMap) {
			diags[i].Position.Line = lineMap[line]
		}
		if diags[i].EndPosition != nil {
			line = diags[i].EndPosition.Line
			if line > 0 && line < len(lineMap) {
				diags[i].EndPosition.Line = lineMap[line]
			}
		}
	}
}

func finishNodeIdentities(doc *ast.AST, parseDiags, markerDiags []diagnostics.Diagnostic, pending []pendingNodeID, lineMap []int, source string, normalizationModified bool) []diagnostics.Diagnostic {
	if doc != nil {
		ast.SetTableContract(doc)
	}
	if len(pending) == 0 && len(markerDiags) == 0 {
		if doc != nil {
			if err := ast.ValidateTableContract(doc); err != nil {
				parseDiags = append(parseDiags, diagnostics.NewError(err.Error(), doc.GetPosition(), "table-identity"))
			}
		}
		return parseDiags
	}
	if normalizationModified && len(pending) > 0 {
		return append(append(parseDiags, markerDiags...), diagnostics.NewError("node-id association is ambiguous after source normalization; use canonical source or strict mode", diagnostics.NewPosition(pending[0].line, 1), "identity"))
	}
	bound := bindNodeIDs(doc, pending, source)
	restoreNodePositions(doc, lineMap)
	restoreDiagnosticPositions(parseDiags, lineMap)
	parseDiags = append(parseDiags, markerDiags...)
	parseDiags = append(parseDiags, bound...)
	parseDiags = append(parseDiags, ast.ValidateNodeIDs(doc)...)
	if err := ast.ValidateTableContract(doc); err != nil {
		parseDiags = append(parseDiags, diagnostics.NewError(err.Error(), doc.GetPosition(), "table-identity"))
	}
	return parseDiags
}
