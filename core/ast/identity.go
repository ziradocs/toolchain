// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"fmt"
	"regexp"
	"strings"

	"go.ziradocs.com/core/v2/diagnostics"
)

// NodeID is an explicit editorial identity. It is deliberately separate from
// table/equation/heading reference labels and their derived HTML anchors.
// The restricted alphabet also makes the source directive unambiguous.
var nodeIDPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9._-]{0,127}$`)

func ValidNodeID(id string) bool { return nodeIDPattern.MatchString(id) }

// IdentityNode is implemented by every AST node embedding BaseNode.
type IdentityNode interface {
	Node
	GetNodeID() string
	SetNodeID(string)
}

// ValidateNodeIDs checks the final tree, including nested nodes. Call after
// transforms too: filters may create nodes, remove nodes, or edit identities.
func ValidateNodeIDs(doc *AST) []diagnostics.Diagnostic {
	type firstUse struct {
		pos  diagnostics.Position
		what string
	}
	seen := make(map[string]firstUse)
	var result []diagnostics.Diagnostic
	check := func(id string, pos diagnostics.Position, what string) {
		if id == "" {
			return
		}
		if !ValidNodeID(id) {
			result = append(result, diagnostics.NewError(fmt.Sprintf("invalid nodeId %q: use 1-128 ASCII letters, digits, '.', '_' or '-', starting with a letter", id), pos, "identity"))
		} else if first, exists := seen[id]; exists {
			// El texto no dice en qué línea está el primero: el número se
			// calcula antes de llevar las posiciones a su archivo (@include,
			// node-id) y saldría distinto del real. Lo nombra por lo que es
			// y, para quien lee el diagnóstico completo, un aviso en su
			// posición (que el remapeo lleva a su archivo como cualquier otro).
			result = append(result,
				diagnostics.NewError(fmt.Sprintf("duplicate nodeId %q (also on %s)", id, first.what), pos, "identity"),
				diagnostics.NewInfo(fmt.Sprintf("nodeId %q is first used here; a later node repeats it", id), first.pos, "identity"))
		} else {
			seen[id] = firstUse{pos: pos, what: what}
		}
	}
	_ = Walk(doc, func(node Node) error {
		if identified, ok := node.(IdentityNode); ok {
			check(identified.GetNodeID(), node.GetPosition(), describeNode(node))
		}
		if table, ok := node.(*TableElement); ok && table.HasTableRows() {
			for _, row := range table.TableRows {
				check(row.NodeID, table.GetPosition(), "a row of the table")
				for _, cell := range row.Cells {
					check(cell.NodeID, table.GetPosition(), "a cell of the table")
				}
			}
		}
		return nil
	})
	return result
}

// describeNode nombra un nodo por su tipo y, si tiene prosa, un extracto de
// su primera línea: `the text "two"`.
func describeNode(node Node) string {
	var text string
	switch n := node.(type) {
	case *TextElement:
		text = n.Content
	case *HeadingElement:
		text = n.Text
	case *ContentBlock:
		text = n.Title
	case *SpecialBlockElement:
		text = n.Title
		if text == "" {
			text = n.Content
		}
	case *QuoteElement:
		text = n.Content
	case *CodeElement:
		text = n.Content
	case *ImageElement:
		text = n.Source
	}
	kind := strings.ReplaceAll(string(node.GetType()), "_", " ")
	text = strings.TrimSpace(strings.SplitN(strings.TrimSpace(text), "\n", 2)[0])
	if text == "" {
		return "a " + kind + " node"
	}
	if r := []rune(text); len(r) > 40 {
		text = string(r[:40]) + "…"
	}
	return fmt.Sprintf("the %s %q", kind, text)
}
