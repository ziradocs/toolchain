// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"encoding/json"
	"fmt"
	"strings"
)

// UsesTableRows finds opt-in tables, including tables in nested elements.
func UsesTableRows(doc *AST) bool {
	used := false
	_ = Walk(doc, func(n Node) error {
		if t, ok := n.(*TableElement); ok && t.HasTableRows() {
			used = true
		}
		return nil
	})
	return used
}

// SetTableContract derives the table and nested-list source capabilities
// after parsing. JSON/filter ingress validates declarations instead.
func SetTableContract(doc *AST) {
	tables, lists := UsesTableRows(doc), UsesNestedListTypes(doc)
	switch {
	case lists:
		doc.SchemaVersion = SchemaVersion
		doc.Capabilities = []string{NestedListTypesCapability}
		if tables {
			doc.Capabilities = []string{TableRowsCapability, NestedListTypesCapability}
		}
	case tables:
		doc.SchemaVersion = TableSchemaVersion
		doc.Capabilities = []string{TableRowsCapability}
	}
}

func UsesNestedListTypes(doc *AST) bool {
	used := false
	_ = Walk(doc, func(n Node) error {
		if p, ok := n.(*PointItem); ok && p.SubListType != "" {
			used = true
		}
		return nil
	})
	return used
}

// NestedListOwners uses only authored node IDs. It is a filter boundary
// check; without IDs, a filter could silently move a child to another list.
func NestedListOwners(doc *AST) (map[string]string, error) {
	owners := map[string]string{}
	err := Walk(doc, func(n Node) error {
		points, ok := n.(*PointsElement)
		if !ok || !pointListHasType(points.Items) {
			return nil
		}
		if points.GetNodeID() == "" {
			return fmt.Errorf("nested-list filter requires nodeId on points element")
		}
		var visit func([]PointItem, string) error
		visit = func(items []PointItem, owner string) error {
			for _, item := range items {
				if item.GetNodeID() == "" {
					return fmt.Errorf("nested-list filter requires nodeId on every point item")
				}
				owners[item.GetNodeID()] = owner
				if err := visit(item.SubPoints, item.GetNodeID()); err != nil {
					return err
				}
			}
			return nil
		}
		return visit(points.Items, points.GetNodeID())
	})
	return owners, err
}

func pointListHasType(items []PointItem) bool {
	for _, item := range items {
		if item.SubListType != "" || pointListHasType(item.SubPoints) {
			return true
		}
	}
	return false
}

func hasCapabilities(caps []string, expected ...string) bool {
	if len(caps) != len(expected) {
		return false
	}
	seen := make(map[string]bool, len(caps))
	for _, c := range caps {
		if seen[c] {
			return false
		}
		seen[c] = true
	}
	for _, c := range expected {
		if !seen[c] {
			return false
		}
	}
	return true
}

// ValidateTableContract checks both opt-in contracts: version/capability,
// nested child-list types, table identity, and table projections. A filter
// may edit/reorder authored rows but must rederive compatibility views;
// a mismatch is never silently repaired.
func ValidateTableContract(doc *AST) error {
	tables, lists := UsesTableRows(doc), UsesNestedListTypes(doc)
	switch {
	case lists && tables:
		if doc.SchemaVersion != SchemaVersion || !hasCapabilities(doc.Capabilities, TableRowsCapability, NestedListTypesCapability) {
			return fmt.Errorf("nested lists and tableRows require schemaVersion %s and both capabilities", SchemaVersion)
		}
	case lists:
		if doc.SchemaVersion != SchemaVersion || !hasCapabilities(doc.Capabilities, NestedListTypesCapability) {
			return fmt.Errorf("nested lists require schemaVersion %s and capability %s", SchemaVersion, NestedListTypesCapability)
		}
	case tables:
		if doc.SchemaVersion != TableSchemaVersion || !hasCapabilities(doc.Capabilities, TableRowsCapability) {
			return fmt.Errorf("tableRows requires schemaVersion %s and capability %s", TableSchemaVersion, TableRowsCapability)
		}
	default:
		if (doc.SchemaVersion != LegacySchemaVersion && doc.SchemaVersion != PreviousSchemaVersion) || len(doc.Capabilities) != 0 {
			return fmt.Errorf("unsupported AST version/capabilities without extensions: %q %v", doc.SchemaVersion, doc.Capabilities)
		}
	}
	var failure error
	_ = Walk(doc, func(n Node) error {
		if p, ok := n.(*PointsElement); ok && lists && p.ListType != "ordered" && p.ListType != "unordered" {
			failure = fmt.Errorf("invalid PointsElement.listType %q", p.ListType)
			return failure
		}
		if t, ok := n.(*TableElement); ok {
			if err := ValidateTableRows(t); err != nil {
				failure = err
				return err
			}
		}
		if p, ok := n.(*PointItem); ok {
			if (p.SubListType != "" && p.SubListType != "ordered" && p.SubListType != "unordered") || (lists && len(p.SubPoints) > 0 && p.SubListType == "") || (p.SubListType != "" && len(p.SubPoints) == 0) {
				failure = fmt.Errorf("invalid PointItem.subListType: each parent with subPoints requires ordered/unordered and leaf items cannot declare it")
				return failure
			}
		}
		return nil
	})
	if failure != nil {
		return failure
	}
	if issues := ValidateNodeIDs(doc); len(issues) != 0 {
		return fmt.Errorf("%s", issues[0].String())
	}
	return nil
}

// ValidateRawTableContract runs before DecodeAST's permissive Go unmarshal.
// It blocks unknown versions, lost capabilities, malformed rows, and nested
// list fields before unrecognized semantic fields could be discarded.
func ValidateRawTableContract(data []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}
	var version string
	if err := json.Unmarshal(root["schemaVersion"], &version); err != nil {
		return fmt.Errorf("schemaVersion is missing or invalid: %w", err)
	}
	if version != PreviousSchemaVersion && version != LegacySchemaVersion && version != TableSchemaVersion && version != SchemaVersion {
		return fmt.Errorf("unsupported schemaVersion %q", version)
	}
	var caps []string
	rawCaps, capsPresent := root["capabilities"]
	if capsPresent {
		if err := json.Unmarshal(rawCaps, &caps); err != nil {
			return fmt.Errorf("invalid capabilities: %w", err)
		}
	}
	if version == TableSchemaVersion && !hasCapabilities(caps, TableRowsCapability) {
		return fmt.Errorf("schemaVersion %s requires capability %s", TableSchemaVersion, TableRowsCapability)
	}
	if version == SchemaVersion && !hasCapabilities(caps, NestedListTypesCapability) && !hasCapabilities(caps, NestedListTypesCapability, TableRowsCapability) {
		return fmt.Errorf("schemaVersion %s requires %s and optionally %s", SchemaVersion, NestedListTypesCapability, TableRowsCapability)
	}
	if version != SchemaVersion && version != TableSchemaVersion && capsPresent {
		return fmt.Errorf("legacy schemaVersion cannot declare capabilities")
	}
	var whole map[string]any
	if err := json.Unmarshal(data, &whole); err != nil {
		return err
	}
	tableCount, listCount := 0, 0
	var inspectElement func(any) error
	inspectElement = func(value any) error {
		switch v := value.(type) {
		case []any:
			for _, child := range v {
				if err := inspectElement(child); err != nil {
					return err
				}
			}
		case map[string]any:
			if v["type"] == string(NodeTypePoints) && version == SchemaVersion && v["listType"] != "ordered" && v["listType"] != "unordered" {
				return fmt.Errorf("invalid PointsElement.listType %v", v["listType"])
			}
			if v["type"] == string(NodeTypeTable) {
				if rows, has := v["tableRows"]; has {
					tableCount++
					if version != TableSchemaVersion && version != SchemaVersion {
						return fmt.Errorf("tableRows requires schemaVersion %s or %s", TableSchemaVersion, SchemaVersion)
					}
					encoded, _ := json.Marshal(rows)
					var parsed []TableRow
					decoder := json.NewDecoder(strings.NewReader(string(encoded)))
					decoder.DisallowUnknownFields()
					if err := decoder.Decode(&parsed); err != nil {
						return fmt.Errorf("invalid tableRows: %w", err)
					}
				}
			}
			if _, has := v["tableRows"]; has && v["type"] != string(NodeTypeTable) {
				return fmt.Errorf("tableRows on a non-table element")
			}
			if kind, has := v["subListType"]; has {
				if v["type"] != string(NodeTypePointItem) {
					return fmt.Errorf("subListType on a non-point item")
				}
				if kind != "ordered" && kind != "unordered" {
					return fmt.Errorf("invalid subListType %v", kind)
				}
				children, ok := v["subPoints"].([]any)
				if !ok || len(children) == 0 {
					return fmt.Errorf("subListType requires nonempty subPoints")
				}
				listCount++
			}
			if v["type"] == string(NodeTypePointItem) {
				if children, ok := v["subPoints"].([]any); ok && len(children) > 0 && version == SchemaVersion {
					if _, has := v["subListType"]; !has {
						return fmt.Errorf("nested list parent requires subListType")
					}
				}
			}
			for _, key := range []string{"contentBlocks", "elements", "columns", "items", "subPoints"} {
				if child, ok := v[key]; ok {
					if err := inspectElement(child); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err := inspectElement(whole); err != nil {
		return err
	}
	if (tableCount > 0) != (version == TableSchemaVersion || (version == SchemaVersion && hasCapabilities(caps, NestedListTypesCapability, TableRowsCapability))) {
		return fmt.Errorf("tableRows presence does not match schemaVersion/capabilities")
	}
	if (listCount > 0) != (version == SchemaVersion) {
		return fmt.Errorf("subListType presence does not match schemaVersion/capabilities")
	}
	return nil
}

// TableIdentityOwners records document-wide row/cell identity owners without
// using coordinates or content. It supports post-filter preservation checks.
func TableIdentityOwners(doc *AST) map[string]string {
	out := map[string]string{}
	_ = Walk(doc, func(n Node) error {
		t, ok := n.(*TableElement)
		if !ok || !t.HasTableRows() {
			return nil
		}
		owner := t.NodeID
		if owner != "" {
			out[owner] = "table"
		}
		for _, row := range t.TableRows {
			if row.NodeID != "" {
				out[row.NodeID] = "row@" + owner
			}
			for _, cell := range row.Cells {
				if cell.NodeID != "" {
					out[cell.NodeID] = "cell@" + owner + "/" + row.NodeID
				}
			}
		}
		return nil
	})
	return out
}

// FilterTableIdentityReady ensures parent ownership can be checked without
// guessing from row order, cell text, or source coordinates.
func FilterTableIdentityReady(doc *AST) error {
	var failure error
	_ = Walk(doc, func(n Node) error {
		t, ok := n.(*TableElement)
		if !ok || !t.HasTableRows() {
			return nil
		}
		if t.NodeID == "" {
			failure = fmt.Errorf("filtering tableRows requires an explicit table nodeId to verify ownership")
			return failure
		}
		for _, row := range t.TableRows {
			if row.NodeID == "" {
				for _, cell := range row.Cells {
					if cell.NodeID != "" {
						failure = fmt.Errorf("filtering identified cell %q requires its row nodeId to verify ownership", cell.NodeID)
						return failure
					}
				}
			}
		}
		return nil
	})
	return failure
}

// TableIdentityTarget is a portable resolution result. The pointers identify
// authored records, never flattened coordinates. Callers must resolve again
// after a transform that replaces or reorders the document tree.
type TableIdentityTarget struct {
	Kind  string // table, row, cell
	Table *TableElement
	Row   *TableRow
	Cell  *TableRowCell
}

func ResolveTableIdentity(doc *AST, id string) (TableIdentityTarget, bool) {
	var target TableIdentityTarget
	if id == "" {
		return target, false
	}
	_ = Walk(doc, func(n Node) error {
		t, ok := n.(*TableElement)
		if !ok {
			return nil
		}
		if t.NodeID == id {
			target = TableIdentityTarget{Kind: "table", Table: t}
			return nil
		}
		for i := range t.TableRows {
			row := &t.TableRows[i]
			if row.NodeID == id {
				target = TableIdentityTarget{Kind: "row", Table: t, Row: row}
				return nil
			}
			for j := range row.Cells {
				cell := &row.Cells[j]
				if cell.NodeID == id {
					target = TableIdentityTarget{Kind: "cell", Table: t, Row: row, Cell: cell}
					return nil
				}
			}
		}
		return nil
	})
	return target, target.Kind != ""
}
