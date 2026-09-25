// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"fmt"
	"regexp"

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
	seen := make(map[string]diagnostics.Position)
	var result []diagnostics.Diagnostic
	check := func(id string, pos diagnostics.Position) {
		if id == "" {
			return
		}
		if !ValidNodeID(id) {
			result = append(result, diagnostics.NewError(fmt.Sprintf("invalid nodeId %q: use 1-128 ASCII letters, digits, '.', '_' or '-', starting with a letter", id), pos, "identity"))
		} else if first, exists := seen[id]; exists {
			result = append(result, diagnostics.NewError(fmt.Sprintf("duplicate nodeId %q (first at %d:%d)", id, first.Line, first.Column), pos, "identity"))
		} else {
			seen[id] = pos
		}
	}
	_ = Walk(doc, func(node Node) error {
		if identified, ok := node.(IdentityNode); ok {
			check(identified.GetNodeID(), node.GetPosition())
		}
		if table, ok := node.(*TableElement); ok && table.HasTableRows() {
			for _, row := range table.TableRows {
				check(row.NodeID, table.GetPosition())
				for _, cell := range row.Cells {
					check(cell.NodeID, table.GetPosition())
				}
			}
		}
		return nil
	})
	return result
}
