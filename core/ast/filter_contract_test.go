package ast

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
)

func pruneFixture(t *testing.T, elements ...Element) []byte {
	t.Helper()
	pos := diagnostics.NewPosition(1, 1)
	doc := NewAST(pos)
	block := NewContentBlock(pos, "content")
	block.Elements = elements
	doc.ContentBlocks = []ContentBlock{*block}
	doc.SchemaVersion = ChartContentSchemaVersion
	doc.Capabilities = []string{MathSourceCapability, ChartContentCapability}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestPruneUnusedCapabilities(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	unit := "MXN"
	chart := &ChartElement{BaseNode: NewBaseNode(NodeTypeChart, pos), ChartType: "bar", Unit: &unit}
	image := NewImageElement(pos, "fig.png", "figura")

	t.Run("nada que quitar deja los bytes intactos", func(t *testing.T) {
		data := pruneFixture(t, NewMathElement(pos, "x"), chart)
		out, dropped := PruneUnusedCapabilities(data)
		if !bytes.Equal(out, data) || len(dropped) != 0 {
			t.Fatalf("cambió: %v", dropped)
		}
	})

	t.Run("quita la de la gráfica y baja la versión", func(t *testing.T) {
		out, dropped := PruneUnusedCapabilities(pruneFixture(t, NewMathElement(pos, "x"), image))
		if !reflect.DeepEqual(dropped, []string{ChartContentCapability}) {
			t.Fatalf("dropped = %v", dropped)
		}
		doc, err := DecodeAST(out)
		if err != nil {
			t.Fatal(err)
		}
		if doc.SchemaVersion != MathSourceSchemaVersion || !reflect.DeepEqual(doc.Capabilities, []string{MathSourceCapability}) {
			t.Fatalf("contrato: %s %v", doc.SchemaVersion, doc.Capabilities)
		}
	})

	t.Run("sin fórmulas ni gráficas queda legado y sin la clave", func(t *testing.T) {
		out, dropped := PruneUnusedCapabilities(pruneFixture(t, image))
		if !reflect.DeepEqual(dropped, []string{MathSourceCapability, ChartContentCapability}) {
			t.Fatalf("dropped = %v", dropped)
		}
		var root map[string]json.RawMessage
		if err := json.Unmarshal(out, &root); err != nil {
			t.Fatal(err)
		}
		if _, has := root["capabilities"]; has {
			t.Fatal("una versión legada no lleva capabilities")
		}
		doc, err := DecodeAST(out)
		if err != nil {
			t.Fatal(err)
		}
		if doc.SchemaVersion != LegacySchemaVersion {
			t.Fatalf("version = %s", doc.SchemaVersion)
		}
	})

	t.Run("nunca agrega lo que se usa sin declarar", func(t *testing.T) {
		titled := NewImageElement(pos, "fig.png", "figura")
		titled.Title = "Título"
		out, dropped := PruneUnusedCapabilities(pruneFixture(t, NewMathElement(pos, "x"), titled))
		if !reflect.DeepEqual(dropped, []string{ChartContentCapability}) {
			t.Fatalf("dropped = %v", dropped)
		}
		if bytes.Contains(out, []byte(ImageTitleCapability)) {
			t.Fatal("agregó image-title-v1")
		}
		if _, err := DecodeAST(out); err == nil {
			t.Fatal("un título sin image-title-v1 pasó el decode")
		}
	})

	t.Run("una declaración mal formada no se toca", func(t *testing.T) {
		data := pruneFixture(t, image)
		bad := bytes.Replace(data, []byte(`"schemaVersion":"2.23.0"`), []byte(`"schemaVersion":"2.22.0"`), 1)
		if bytes.Equal(bad, data) {
			t.Fatal("fixture")
		}
		out, dropped := PruneUnusedCapabilities(bad)
		if !bytes.Equal(out, bad) || len(dropped) != 0 {
			t.Fatalf("cambió: %v", dropped)
		}
	})
}
