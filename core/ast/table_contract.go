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

// SetTableContract is used only after parsing authored source. JSON/filter
// ingress must validate the declared version and capabilities instead.
func SetTableContract(doc *AST) {
	if UsesTableRows(doc) {
		doc.SchemaVersion = SchemaVersion
		doc.Capabilities = []string{TableRowsCapability}
	}
}

// ValidateTableContract checks version, capability, identity and every
// projection. A filter may edit/reorder authored rows, but must rederive its
// compatibility views explicitly; a mismatch is never silently repaired.
func ValidateTableContract(doc *AST) error {
	used := UsesTableRows(doc)
	if used {
		if doc.SchemaVersion != SchemaVersion || len(doc.Capabilities) != 1 || doc.Capabilities[0] != TableRowsCapability {
			return fmt.Errorf("tableRows requires schemaVersion %s and capability %s", SchemaVersion, TableRowsCapability)
		}
	} else if doc.SchemaVersion != LegacySchemaVersion || len(doc.Capabilities) != 0 {
		return fmt.Errorf("unsupported AST version/capabilities without tableRows: %q %v", doc.SchemaVersion, doc.Capabilities)
	}
	var failure error
	_ = Walk(doc, func(n Node) error {
		if t, ok := n.(*TableElement); ok {
			if err := ValidateTableRows(t); err != nil {
				failure = err
				return err
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
// It blocks unknown versions, lost capabilities, and malformed row records
// before any unrecognized identity-bearing field could be discarded.
func ValidateRawTableContract(data []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}
	var version string
	if err := json.Unmarshal(root["schemaVersion"], &version); err != nil {
		return fmt.Errorf("schemaVersion is missing or invalid: %w", err)
	}
	if version != LegacySchemaVersion && version != SchemaVersion {
		return fmt.Errorf("unsupported schemaVersion %q", version)
	}
	var caps []string
	if raw, ok := root["capabilities"]; ok {
		if err := json.Unmarshal(raw, &caps); err != nil {
			return fmt.Errorf("invalid capabilities: %w", err)
		}
	}
	if version == SchemaVersion && (len(caps) != 1 || caps[0] != TableRowsCapability) {
		return fmt.Errorf("schemaVersion %s requires capability %s", SchemaVersion, TableRowsCapability)
	}
	if version == LegacySchemaVersion && len(caps) != 0 {
		return fmt.Errorf("legacy schemaVersion cannot declare capabilities")
	}
	var whole any
	if err := json.Unmarshal(data, &whole); err != nil {
		return err
	}
	count := 0
	var inspect func(any) error
	inspect = func(value any) error {
		switch v := value.(type) {
		case []any:
			for _, child := range v {
				if err := inspect(child); err != nil { return err }
			}
		case map[string]any:
			if v["type"] == string(NodeTypeTable) {
				if rows, has := v["tableRows"]; has {
					count++
					if version != SchemaVersion {
						return fmt.Errorf("tableRows requires schemaVersion %s", SchemaVersion)
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
			for _, child := range v {
				if err := inspect(child); err != nil { return err }
			}
		}
		return nil
	}
	if err := inspect(whole); err != nil { return err }
	if version == SchemaVersion && count == 0 {
		return fmt.Errorf("schemaVersion %s declares %s without tableRows", SchemaVersion, TableRowsCapability)
	}
	return nil
}

// TableIdentityOwners records document-wide row/cell identity owners without
// using coordinates or content. It supports post-filter preservation checks.
func TableIdentityOwners(doc *AST) map[string]string {
	out := map[string]string{}
	_ = Walk(doc, func(n Node) error {
		t, ok := n.(*TableElement)
		if !ok || !t.HasTableRows() { return nil }
		owner := t.NodeID
		if owner != "" { out[owner] = "table" }
		for _, row := range t.TableRows {
			if row.NodeID != "" { out[row.NodeID] = "row@" + owner }
			for _, cell := range row.Cells {
				if cell.NodeID != "" { out[cell.NodeID] = "cell@" + owner + "/" + row.NodeID }
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
		if !ok || !t.HasTableRows() { return nil }
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
	Kind string // table, row, cell
	Table *TableElement
	Row *TableRow
	Cell *TableRowCell
}

func ResolveTableIdentity(doc *AST, id string) (TableIdentityTarget, bool) {
	var target TableIdentityTarget
	if id == "" { return target, false }
	_ = Walk(doc, func(n Node) error {
		t, ok := n.(*TableElement)
		if !ok { return nil }
		if t.NodeID == id { target = TableIdentityTarget{Kind: "table", Table: t}; return nil }
		for i := range t.TableRows {
			row := &t.TableRows[i]
			if row.NodeID == id { target = TableIdentityTarget{Kind: "row", Table: t, Row: row}; return nil }
			for j := range row.Cells {
				cell := &row.Cells[j]
				if cell.NodeID == id { target = TableIdentityTarget{Kind: "cell", Table: t, Row: row, Cell: cell}; return nil }
			}
		}
		return nil
	})
	return target, target.Kind != ""
}
