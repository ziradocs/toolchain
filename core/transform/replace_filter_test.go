// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package transform

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// replaceFilterSource es un filtro real, como el que escribiría quien
// exporta un documento con figuras ya dibujadas: cambia cada gráfica y cada
// fórmula por una imagen y devuelve la declaración raíz (schemaVersion y
// capabilities) tal como llegó, sin recalcularla.
const replaceFilterSource = `package main

import (
	"encoding/json"
	"io"
	"os"
)

func replace(v any) {
	switch x := v.(type) {
	case []any:
		for i, child := range x {
			if m, ok := child.(map[string]any); ok && (m["type"] == "chart" || m["type"] == "math") {
				x[i] = map[string]any{"type": "image", "position": m["position"], "endPosition": m["endPosition"], "source": "fig.png", "alt": "figura"}
				continue
			}
			replace(child)
		}
	case map[string]any:
		for _, key := range []string{"contentBlocks", "elements", "columns"} {
			replace(x[key])
		}
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--ziradocs-capabilities" {
		os.Stdout.WriteString(os.Getenv("HANDSHAKE"))
		return
	}
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		panic(err)
	}
	replace(doc)
	out, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	os.Stdout.Write(out)
}
`

// buildReplaceFilter compila replaceFilterSource y devuelve un script que lo
// ejecuta respondiendo al handshake con todas las versiones y capabilities
// que este core conoce.
func buildReplaceFilter(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain no disponible en PATH")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	if err := os.WriteFile(src, []byte(replaceFilterSource), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "replace-filter")
	cmd := exec.Command("go", "build", "-o", bin, src)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compilando el filtro de prueba: %v\n%s", err, out)
	}
	handshake, err := json.Marshal(map[string][]string{
		"astSchemaVersions": {ast.TableSchemaVersion, ast.NestedListSchemaVersion, ast.TypedHeadingsSchemaVersion, ast.MediaFigureSchemaVersion, ast.CodeFilenameSchemaVersion, ast.QuizPollResultsSchemaVersion, ast.ListStartSchemaVersion, ast.MathSourceSchemaVersion, ast.ChartContentSchemaVersion, ast.ContentMarkdownSchemaVersion, ast.TableAlignSchemaVersion, ast.ImageTitleSchemaVersion},
		"features":          ast.KnownCapabilities(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return writeFilter(t, "HANDSHAKE='"+string(handshake)+"' exec "+bin+` "$@"`)
}

// figuresFixture: una gráfica con unidad (chart-content-v1) y dos fórmulas,
// una con label, bajo la política math-source-v1. Schema 2.23.0.
func figuresFixture(withMathSource bool) *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	labeled := ast.NewMathElement(pos, "  E = mc^2")
	labeled.Label, labeled.Number = "eq:a", 1
	unit := "MXN"
	chart := &ast.ChartElement{BaseNode: ast.NewBaseNode(ast.NodeTypeChart, pos), ChartType: "bar", Unit: &unit}
	block.Elements = append(block.Elements, labeled, ast.NewMathElement(pos, "a^2 + b^2 = c^2"), chart)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	if withMathSource {
		doc.Capabilities = []string{ast.MathSourceCapability}
	}
	ast.SetTableContract(doc)
	return doc
}

func TestRunFilters_ReplacedFiguresRederiveContract(t *testing.T) {
	filter := buildReplaceFilter(t)
	for _, withMathSource := range []bool{false, true} {
		in := figuresFixture(withMathSource)
		want := []string{ast.ChartContentCapability}
		if withMathSource {
			want = []string{ast.MathSourceCapability, ast.ChartContentCapability}
		}
		if strings.Join(in.Capabilities, ",") != strings.Join(want, ",") || in.SchemaVersion != ast.ChartContentSchemaVersion {
			t.Fatalf("fixture: %s %v", in.SchemaVersion, in.Capabilities)
		}
		out, err := RunFilters(in, []string{filter}, 10*time.Second)
		if err != nil {
			t.Fatalf("mathSource=%v: %v", withMathSource, err)
		}
		if out.SchemaVersion != ast.LegacySchemaVersion || len(out.Capabilities) != 0 {
			t.Fatalf("mathSource=%v: el contrato no se recalculó: %s %v", withMathSource, out.SchemaVersion, out.Capabilities)
		}
		for _, e := range out.ContentBlocks[0].Elements {
			if _, ok := e.(*ast.ImageElement); !ok {
				t.Fatalf("quedó un %T sin reemplazar", e)
			}
		}
		if err := ast.ValidateTableContract(out); err != nil {
			t.Fatalf("el AST recalculado no valida: %v", err)
		}
	}
}

// Un filtro que quita la política mientras quedan fórmulas sigue siendo
// rechazado: es la pérdida que la regla de math-source-v1 evita.
func TestRunFilters_MathSourcePolicyStillProtectedWhileMathRemains(t *testing.T) {
	handshake := `if [ "$1" = "--ziradocs-capabilities" ]; then
echo '{"astSchemaVersions":["2.23.0"],"features":["math-source-v1","chart-content-v1"]}'
exit 0
fi
`
	// Quita math-source-v1 y deja la versión en la de chart-content-v1: una
	// declaración bien formada, sin la política, con las fórmulas intactas.
	script := `sed 's/"math-source-v1",//g'`
	_, err := RunFilters(figuresFixture(true), []string{writeFilter(t, handshake+script)}, 5*time.Second)
	if err == nil || !strings.Contains(err.Error(), "math-source-v1 was removed while equations remain") {
		t.Fatalf("pérdida de la política aceptada: %v", err)
	}
	// La misma política se conserva si el filtro no toca nada.
	out, err := RunFilters(figuresFixture(true), []string{writeFilter(t, handshake+"cat")}, 5*time.Second)
	if err != nil || !ast.UsesMathSource(out) {
		t.Fatalf("identidad: %v", err)
	}
}

// Un builtin que reemplaza todas las fórmulas tampoco necesita conservar una
// política que ya no aplica a nada.
func TestRunBuiltins_MathSourceDroppedWithAllMath(t *testing.T) {
	doc := figuresFixture(true)
	out, err := RunBuiltins(doc, []Transform{func(d *ast.AST) (*ast.AST, error) {
		d.ContentBlocks[0].Elements = d.ContentBlocks[0].Elements[2:]
		d.Capabilities = []string{ast.ChartContentCapability}
		return d, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if hasMathElement(out) {
		t.Fatal("fixture")
	}
}
