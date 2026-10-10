// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import "fmt"

// UsesListStart detects serialized ordinals at any list depth.
func UsesListStart(doc *AST) bool {
	used := false
	_ = Walk(doc, func(n Node) error {
		switch e := n.(type) {
		case *PointsElement:
			used = used || e.Start != nil
		case *PointItem:
			used = used || e.SubListStart != nil
		}
		return nil
	})
	return used
}

// UsesMathSource is a document policy, including programmatically constructed
// ASTs. No hidden per-element marker is needed to preserve literal content.
func UsesMathSource(doc *AST) bool {
	if doc == nil {
		return false
	}
	declared := SourceDeclaresCapability(doc.FrontMatter, MathSourceCapability)
	for _, c := range doc.Capabilities {
		declared = declared || c == MathSourceCapability
	}
	if !declared {
		return false
	}
	used := false
	_ = Walk(doc, func(n Node) error {
		if _, ok := n.(*MathElement); ok {
			used = true
		}
		return nil
	})
	return used
}

func validateListStart(start *int64, kind string, count int) error {
	if start == nil {
		return nil
	}
	if kind != "ordered" || *start < 1 || *start > MaxListStart {
		return fmt.Errorf("list start requires an ordered list and a positive JSON safe integer")
	}
	if count > 0 && int64(count-1) > MaxListStart-*start {
		return fmt.Errorf("list ordinals exceed the JSON safe integer range")
	}
	return nil
}

// ListStartFingerprints protects ordinals and the ownership of numbered lists
// across external filters while permitting edits to item text and order.
func ListStartFingerprints(doc *AST) (map[string]NestedListFingerprint, error) {
	out := map[string]NestedListFingerprint{}
	err := Walk(doc, func(n Node) error {
		p, ok := n.(*PointsElement)
		if !ok || p.Start == nil && !pointListHasStart(p.Items) {
			return nil
		}
		if p.NodeID == "" {
			return fmt.Errorf("list-start filter requires nodeId on points element")
		}
		out[p.NodeID] = NestedListFingerprint{ListType: p.ListType, Start: p.Start}
		var visit func([]PointItem, string) error
		visit = func(items []PointItem, owner string) error {
			for _, item := range items {
				if item.NodeID == "" {
					return fmt.Errorf("list-start filter requires nodeId on every point item")
				}
				out[item.NodeID] = NestedListFingerprint{OwnerID: owner, SubListType: item.SubListType, Start: item.SubListStart}
				if err := visit(item.SubPoints, item.NodeID); err != nil {
					return err
				}
			}
			return nil
		}
		return visit(p.Items, p.NodeID)
	})
	return out, err
}
func pointListHasStart(items []PointItem) bool {
	for _, p := range items {
		if p.SubListStart != nil || pointListHasStart(p.SubPoints) {
			return true
		}
	}
	return false
}
