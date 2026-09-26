// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"fmt"
	"sort"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

// formatNodeIDs annotates the canonical source only after verifying that the
// formatter preserved the AST traversal shape. This is serialization of IDs
// already present in the AST; author edits are never matched by position,
// content, or hash. A formatter that cannot represent a nested node fails
// rather than silently moving its identity to another node.
func formatNodeIDs(doc *ast.AST, source string, document bool) (string, error) {
	if ast.UsesTableRows(doc) || doc.SchemaVersion == ast.SchemaVersion || len(doc.Capabilities) > 0 {
		if err := ast.ValidateTableContract(doc); err != nil {
			return "", fmt.Errorf("invalid table contract: %w", err)
		}
	}
	if issues := ast.ValidateNodeIDs(doc); len(issues) != 0 {
		return "", fmt.Errorf("invalid node identities: %s", issues[0].String())
	}
	var identified bool
	_ = ast.Walk(doc, func(n ast.Node) error {
		if a, ok := n.(ast.IdentityNode); ok && a.GetNodeID() != "" {
			identified = true
		}
		return nil
	})
	if !identified {
		return source, nil
	}
	if doc.GetNodeID() != "" {
		return "", fmt.Errorf("document root nodeId cannot be represented in source")
	}

	p := parser.New(util.NewNoop())
	p.SetNormalization(false)
	var parsed *ast.AST
	if document {
		parsed, _ = p.ParseDocument(source, "")
	} else if doc.FrontMatter == nil {
		parsed, _ = parser.NewStrictParser(source, util.NewNoop()).Parse()
	} else {
		parsed, _ = p.Parse(source, "")
	}
	if parsed == nil {
		return "", fmt.Errorf("cannot reparse formatted source to place node IDs")
	}
	var before, after []ast.Node
	_ = ast.Walk(doc, func(n ast.Node) error { before = append(before, n); return nil })
	_ = ast.Walk(parsed, func(n ast.Node) error { after = append(after, n); return nil })
	if len(before) != len(after) {
		return "", fmt.Errorf("formatted source changes AST structure (%d nodes to %d); cannot place node IDs", len(before), len(after))
	}
	lines := strings.Split(source, "\n")
	markers := make(map[int]string)
	for i, old := range before {
		identifiedNode, ok := old.(ast.IdentityNode)
		if !ok || identifiedNode.GetNodeID() == "" {
			continue
		}
		if old.GetType() != after[i].GetType() {
			return "", fmt.Errorf("formatted source changes node type at traversal index %d (%s to %s)", i, old.GetType(), after[i].GetType())
		}
		line := after[i].GetPosition().Line
		if line < 1 || line > len(lines) || markers[line] != "" {
			return "", fmt.Errorf("formatted source has ambiguous position for nodeId %q", identifiedNode.GetNodeID())
		}
		indent := lines[line-1][:len(lines[line-1])-len(strings.TrimLeft(lines[line-1], " \t"))]
		markers[line] = indent + "<!-- node-id: " + identifiedNode.GetNodeID() + " -->"
	}
	positions := make([]int, 0, len(markers))
	for line := range markers {
		positions = append(positions, line)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(positions)))
	for _, line := range positions {
		idx := line - 1
		lines = append(lines[:idx], append([]string{markers[line]}, lines[idx:]...)...)
	}
	return strings.Join(lines, "\n"), nil
}
