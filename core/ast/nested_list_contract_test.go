package ast

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"go.ziradocs.com/core/v2/diagnostics"
)

func nestedListFixture(withTable bool) *AST {
	pos := diagnostics.NewPosition(1, 1)
	var doc *AST
	if withTable {
		doc = contractFixture()
	} else {
		doc = NewAST(pos)
		doc.ContentBlocks = []ContentBlock{*NewContentBlock(pos, "content")}
	}
	points := NewPointsElement(pos)
	points.NodeID = "PointsA"
	points.ListType = "ordered"
	parent := NewPointItem(pos, "ParentA")
	parent.NodeID = "ParentA"
	parent.SubListType = "unordered"
	child := NewPointItem(pos, "ChildA")
	child.NodeID = "ChildA"
	parent.SubPoints = append(parent.SubPoints, *child)
	points.Items = append(points.Items, *parent)
	doc.ContentBlocks[0].Elements = append(doc.ContentBlocks[0].Elements, points)
	SetTableContract(doc)
	return doc
}

func TestNestedListContractVersionMatrix(t *testing.T) {
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(filepath.Join("..", "..", "schema", "ast.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, doc := range []*AST{nestedListFixture(false), nestedListFixture(true)} {
		if err := ValidateTableContract(doc); err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(doc)
		if _, err := DecodeAST(data); err != nil {
			t.Fatal(err)
		}
		var value any
		_ = json.Unmarshal(data, &value)
		if err := schema.Validate(value); err != nil {
			t.Fatal(err)
		}
		if doc.SchemaVersion != SchemaVersion {
			t.Fatalf("wrong version %s", doc.SchemaVersion)
		}
		caps := doc.Capabilities
		if len(caps) == 0 || caps[len(caps)-1] != NestedListTypesCapability {
			t.Fatalf("wrong capabilities %v", caps)
		}
		if len(caps) == 2 {
			root := value.(map[string]any)
			root["capabilities"] = []any{NestedListTypesCapability, TableRowsCapability}
			reordered, _ := json.Marshal(root)
			if _, err := DecodeAST(reordered); err != nil {
				t.Fatalf("capability order became semantic: %v", err)
			}
			if err := schema.Validate(root); err != nil {
				t.Fatalf("schema rejected reordered capabilities: %v", err)
			}
		}
	}
}

func TestNestedListContractRejectsSilentLoss(t *testing.T) {
	compiler := jsonschema.NewCompiler()
	schema, err := compiler.Compile(filepath.Join("..", "..", "schema", "ast.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	doc := nestedListFixture(false)
	data, _ := json.Marshal(doc)
	var base map[string]any
	_ = json.Unmarshal(data, &base)
	check := func(name string, mutate func(map[string]any)) {
		t.Helper()
		var root map[string]any
		copyData, _ := json.Marshal(base)
		_ = json.Unmarshal(copyData, &root)
		mutate(root)
		candidate, _ := json.Marshal(root)
		if _, err := DecodeAST(candidate); err == nil {
			t.Errorf("decoder accepted %s", name)
		}
		if err := schema.Validate(root); err == nil {
			t.Errorf("schema accepted %s", name)
		}
	}
	item := func(root map[string]any) map[string]any {
		return root["contentBlocks"].([]any)[0].(map[string]any)["elements"].([]any)[0].(map[string]any)["items"].([]any)[0].(map[string]any)
	}
	check("missing capability", func(root map[string]any) { delete(root, "capabilities") })
	check("unknown capability", func(root map[string]any) { root["capabilities"] = []any{"unknown"} })
	check("duplicate capability", func(root map[string]any) {
		root["capabilities"] = []any{NestedListTypesCapability, NestedListTypesCapability}
	})
	check("legacy version", func(root map[string]any) { root["schemaVersion"] = LegacySchemaVersion; delete(root, "capabilities") })
	check("table version", func(root map[string]any) {
		root["schemaVersion"] = TableSchemaVersion
		root["capabilities"] = []any{TableRowsCapability}
	})
	check("missing type", func(root map[string]any) { delete(item(root), "subListType") })
	check("invalid type", func(root map[string]any) { item(root)["subListType"] = "numbered" })
	check("invalid outer type", func(root map[string]any) {
		root["contentBlocks"].([]any)[0].(map[string]any)["elements"].([]any)[0].(map[string]any)["listType"] = "numbered"
	})
	check("orphan type", func(root map[string]any) { delete(item(root), "subPoints") })
}
