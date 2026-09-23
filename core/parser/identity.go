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

// A standalone comment attaches to the very next nonblank source line. The
// line is blanked, rather than removed, so every parser retains its original
// source positions. Ordinary comments remain ordinary source text.
var nodeIDDirective = regexp.MustCompile(`^<!-- node-id: ([^[:space:]]+) -->$`)

type pendingNodeID struct {
	id     string
	line   int
	target int
}

func stripNodeIDDirectives(source string) (string, []pendingNodeID, []diagnostics.Diagnostic) {
	lines := strings.Split(source, "\n")
	var pending []pendingNodeID
	var diags []diagnostics.Diagnostic
	literal := ""
	codeIndent := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if literal == "code" && trimmed != "" && indent < codeIndent {
			literal = ""
		}
		if literal != "" {
			switch literal {
			case "fence":
				if trimmed == "```" {
					literal = ""
				}
			case "diagram":
				if trimmed == "<<end>>" {
					literal = ""
				}
			case "group":
				if trimmed == ":::" {
					literal = ""
				}
			}
			if literal != "" || trimmed == "```" || trimmed == "<<end>>" || trimmed == ":::" {
				continue
			}
		}
		if strings.HasPrefix(trimmed, "```") {
			literal = "fence"
			continue
		}
		if trimmed == "<<mermaid>>" || trimmed == "<<plantuml>>" || trimmed == "<<math>>" || trimmed == "<<map>>" || strings.HasPrefix(trimmed, "<<chart:") {
			literal = "diagram"
			continue
		}
		if strings.HasPrefix(trimmed, ":::code-group") {
			literal = "group"
			continue
		}
		if trimmed == "CODE" || strings.HasPrefix(trimmed, "CODE ") {
			literal = "code"
			codeIndent = indent + 1
			continue
		}
		if !strings.HasPrefix(trimmed, "<!-- node-id:") {
			continue
		}
		pos := diagnostics.NewPosition(i+1, indent+1)
		m := nodeIDDirective.FindStringSubmatch(trimmed)
		if m == nil || !ast.ValidNodeID(m[1]) {
			diags = append(diags, diagnostics.NewError("invalid node-id directive: expected <!-- node-id: Name --> (1-128 ASCII letters, digits, '.', '_' or '-', starting with a letter)", pos, "identity"))
			lines[i] = ""
			continue
		}
		pending = append(pending, pendingNodeID{m[1], i + 1, 0})
		lines[i] = ""
	}
	for i := range pending {
		target := pending[i].line
		for target < len(lines) && strings.TrimSpace(lines[target]) == "" {
			target++
		}
		pending[i].target = target + 1
	}
	return strings.Join(lines, "\n"), pending, diags
}

func bindNodeIDs(doc *ast.AST, pending []pendingNodeID) []diagnostics.Diagnostic {
	var diags []diagnostics.Diagnostic
	if doc == nil {
		return diags
	}
	byLine := make(map[int][]ast.IdentityNode)
	_ = ast.Walk(doc, func(node ast.Node) error {
		if n, ok := node.(ast.IdentityNode); ok && node != doc {
			byLine[node.GetPosition().Line] = append(byLine[node.GetPosition().Line], n)
		}
		return nil
	})
	for _, marker := range pending {
		pos := diagnostics.NewPosition(marker.line, 1)
		matches := byLine[marker.target]
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
	diags = append(diags, ast.ValidateNodeIDs(doc)...)
	return diags
}
