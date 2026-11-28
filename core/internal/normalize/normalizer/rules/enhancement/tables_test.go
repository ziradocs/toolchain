// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"strings"
	"testing"
	"time"
)

// TestTablesRule_Apply_DoesNotRewriteJSONInsideCodeFence cubre issue #204:
// isInSpecialBlock no reconocía fences de triple backtick (```chart,
// ```map, etc.), así que líneas "key": value dentro de un fence se
// reescribían como tabla Markdown.
func TestTablesRule_Apply_DoesNotRewriteJSONInsideCodeFence(t *testing.T) {
	rule := NewTablesRule()

	tests := []struct {
		name  string
		input string
	}{
		{
			name: "chart fence with 3+ consecutive key:value JSON lines",
			input: "# Chart\n\n```chart\n{\n" +
				"  \"type\": \"bar\",\n" +
				"  \"zoom\": 12,\n" +
				"  \"title\": \"Sales\"\n" +
				"}\n```\n",
		},
		{
			// Regression test para el hallazgo de code-review sobre el fix
			// original (PR de issue #204): una implementación ingenua de
			// isInCodeFence (toggle por prefix match en vez de exact match
			// para el cierre) trataba CUALQUIER línea que empezara con
			// "```" -- incluida una línea de CONTENIDO dentro del fence que
			// documenta su propia sintaxis -- como el cierre, cerrando el
			// fence prematuramente y dejando las líneas key:value
			// siguientes vulnerables a la reescritura de tabla otra vez.
			// El parser real (internal/elements/code.go, parseFlexCode)
			// solo cierra con un "```" EXACTO tras trim -- esta regla debe
			// espejar esa asimetría exactamente.
			name: "fence content line starting with triple backticks does not close the fence early",
			input: "# Chart\n\n```chart\n{\n" +
				"  \"note\": \"use ```json for inline examples\",\n" +
				"  \"type\": \"bar\",\n" +
				"  \"zoom\": 12,\n" +
				"  \"title\": \"Sales\"\n" +
				"}\n```\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rule.Apply(tc.input)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if got != tc.input {
				t.Fatalf("Apply() rewrote content inside a code fence, want no-op\n--- input ---\n%s\n--- got ---\n%s", tc.input, got)
			}
		})
	}
}

// TestTablesRule_Apply_StillRewritesTableOutsideFence confirma que el fix
// de isInCodeFence no desactivó la funcionalidad real de la regla: 3+
// líneas key:value FUERA de cualquier fence siguen convirtiéndose en tabla.
func TestTablesRule_Apply_StillRewritesTableOutsideFence(t *testing.T) {
	rule := NewTablesRule()
	input := "# Config\n\nname: value1\ntype: value2\nowner: value3\n"

	got, err := rule.Apply(input)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got == input {
		t.Fatalf("Apply() left 3+ key:value lines outside any fence unchanged, want a Markdown table rewrite:\n%s", got)
	}
}

// TestTablesRule_Apply_ContainerFenceIsABoundary: las líneas ":::" (apertura
// de "::: grid"/"::: column"/":::note" y el cierre a secas) son frontera de
// bloque, no datos de tabla. Antes, una tabla markdown pegada a "::: column"
// se reescribía junto con los marcadores (cada uno partido por ":" en una
// fila falsa) y el grid desaparecía.
func TestTablesRule_Apply_ContainerFenceIsABoundary(t *testing.T) {
	rule := NewTablesRule()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "tabla markdown pegada a ::: column, sin líneas en blanco",
			input: "# Slide\n\n::: grid\n::: column\n| A | B |\n|---|---|\n| 1 | 2 |\n:::\n:::\n",
		},
		{
			name:  "tabla markdown pegada a un bloque :::note",
			input: "# Slide\n\n:::note\n| A | B |\n|---|---|\n| 1 | 2 |\n:::\n",
		},
		{
			// Las filas con pipes no se reescriben por sí solas; lo que
			// antes las arrastraba era la ventana que arrancaba en
			// "::: column" y llegaba hasta el ":::" de cierre.
			name:  "filas con pipes sin separador entre ::: column y :::",
			input: "# Slide\n\n::: column\n| a | b |\n| c | d |\n| e | f |\n:::\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rule.Apply(tc.input)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if got != tc.input {
				t.Fatalf("Apply() reescribió contenido con marcadores :::, quería no-op\n--- input ---\n%s\n--- got ---\n%s", tc.input, got)
			}
		})
	}
}

// TestTablesRule_Apply_ContainerFenceClosingIsNotARow: cuando la regla SÍ
// convierte (3+ líneas clave:valor dentro de un bloque), la ventana termina
// en el ":::" de cierre y este no se convierte en la fila falsa ["", "::"].
func TestTablesRule_Apply_ContainerFenceClosingIsNotARow(t *testing.T) {
	rule := NewTablesRule()

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "lista clave:valor pegada al ::: de cierre",
			input: "# Slide\n\n::: column\nname: value1\ntype: value2\nowner: value3\n:::\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := rule.Apply(tc.input)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if got == tc.input {
				t.Fatalf("Apply() dejó sin tocar las líneas de datos, quería la tabla:\n%s", got)
			}
			if !strings.Contains(got, "\n:::\n") && !strings.HasSuffix(got, "\n:::") {
				t.Errorf("Apply() se comió el ::: de cierre:\n%s", got)
			}
			if strings.Contains(got, "| :: |") || strings.Contains(got, "|  | :: |") {
				t.Errorf("Apply() convirtió el ::: en una fila de la tabla:\n%s", got)
			}
		})
	}
}

// specialBlockLines da, en una pasada, lo mismo que isInSpecialBlock línea por
// línea (que era cuadrático): cercas, cierres que no son ``` exacto,
// etiquetas <<...>>, líneas vacías y separadores "---", al azar.
func TestSpecialBlockLinesMatchesIsInSpecialBlock(t *testing.T) {
	r := NewTablesRule()
	tokens := []string{"```", "```json", "````", "  ```", "texto", "", "---", "<<chart>>", "<<end>>", "a | b", "x: 1", "<<map", ":::"}
	seed := uint32(1)
	next := func() int {
		seed = seed*1664525 + 1013904223
		return int(seed >> 8)
	}
	for n := 0; n < 2000; n++ {
		lines := make([]string, next()%30)
		for i := range lines {
			lines[i] = tokens[next()%len(tokens)]
		}
		got := specialBlockLines(lines)
		for i := range lines {
			if want := r.isInSpecialBlock(lines, i); got[i] != want {
				t.Fatalf("line %d of %q: specialBlockLines %v, isInSpecialBlock %v", i, lines, got[i], want)
			}
		}
	}
}

// La regla es lineal: 40 000 líneas sin frontmatter, con cercas y prosa, se
// normalizan en un tiempo del orden de un parse. Antes cada línea recorría el
// documento desde el principio.
func TestTablesRuleIsLinear(t *testing.T) {
	var b strings.Builder
	for b.Len() < 40000*12 {
		b.WriteString("Un párrafo.\n\n```js\nx\n```\n\n")
	}
	start := time.Now()
	if _, err := NewTablesRule().Apply(b.String()); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("TablesRule.Apply took %v on %d bytes", d, b.Len())
	}
}

// Una tabla Markdown con guiones en una celda ("| 2019 | --- |") ya es una
// tabla, tenga o no fila separadora: la regla no la reescribe. Desde que esa
// fila es de datos en los dos dialectos, reescribirla le cambiaría las celdas.
func TestTablesRule_Apply_KeepsDashCellRows(t *testing.T) {
	rule := NewTablesRule()
	for _, input := range []string{
		"# S\n\n| Año | Valor |\n| --- | --- |\n| 2019 | --- |\n| a --- b | c |\n",
		"# S\n\n| Año | Valor |\n| 2019 | --- |\n| 2020 | 5 |\n",
		"# S\n\n| 2019 | --- |\n| a --- b | c |\n| :-- | x |\n",
		"# S\n\n:::note\n| Año | Valor |\n| 2019 | --- |\n:::\n",
	} {
		got, err := rule.Apply(input)
		if err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		if got != input {
			t.Errorf("Apply() rewrote a Markdown table:\n%s\n---\n%s", input, got)
		}
	}
}

// Una fila de una tabla Markdown que empieza más arriba no abre una tabla
// nueva. Tres filas de una sola celda ("| r0 |", dos pipes) bajo un
// encabezado más ancho se reemplazaban por una línea vacía (menos de dos
// columnas) o, si la racha llegaba a una fila de tres celdas, por un
// encabezado y un separador nuevos a media tabla.
func TestTablesRule_Apply_KeepsRowsOfAnExistingTable(t *testing.T) {
	rule := NewTablesRule()
	head := "| A | B | C |\n|---|---|---|\n"
	for name, input := range map[string]string{
		"three single-cell rows":               "# S\n\n" + head + "| r0 |\n| r1 |\n| r2 |\n",
		"full row, then single-cell rows":      "# S\n\n" + head + "| 1 | 2 | 3 |\n| r0 |\n| r1 |\n| r2 |\n\nDespues.\n",
		"single-cell rows, then a full row":    "# S\n\n" + head + "| r0 |\n| r1 |\n| r2 |\n| 1 | 2 | 3 |\n",
		"twelve single-cell rows":              "# S\n\n" + head + strings.Repeat("| r |\n", 12),
		"one-column table with three rows":     "# S\n\n| A |\n|---|\n| x |\n| y |\n| z |\n",
		"two-cell rows without a closing pipe": "# S\n\n" + head + "| a | b\n| c | d\n| e | f\n",
		"header without separator":             "# S\n\n| A | B |\n| c | d\n| e | f\n| g | h\n",
		"inside a note block":                  "# S\n\n:::note\n" + head + "| r0 |\n| r1 |\n| r2 |\n:::\n",
		"indented under a list item":           "# S\n\n- uno\n\n  " + strings.ReplaceAll(head, "\n", "\n  ") + "| r0 |\n  | r1 |\n  | r2 |\n",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := rule.Apply(input)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if got != input {
				t.Errorf("Apply() rewrote rows of an existing table:\n%s\n---\n%s", input, got)
			}
		})
	}
}

// Cuando las líneas no dan una tabla de dos columnas o más, la regla las deja
// como están. Antes las reemplazaba por una línea vacía y su texto se perdía.
func TestTablesRule_Apply_KeepsLinesThatMakeNoTable(t *testing.T) {
	rule := NewTablesRule()
	input := "# S\n\nAntes.\n\n| r0 |\n| r1 |\n| r2 |\n\nDespues.\n"
	got, err := rule.Apply(input)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got != input {
		t.Errorf("Apply() rewrote single-cell lines:\n%s\n---\n%s", input, got)
	}
}

// La primera línea de una racha sí abre una tabla: tres filas de dos celdas
// sin separador ni pipe de cierre se completan como antes. Una línea con un
// solo pipe arriba no es de ninguna tabla, así que no cambia eso. Y un
// bloque $$ tampoco es una tabla: la regla convierte ahí como en v2.61.0.
// Que convierta dentro de una fórmula es un defecto aparte,
// ziradocs/toolchain#557: quien lo arregle tiene que cambiar esa aserción.
// Una línea separadora antes del encabezado, que el parser salta, tampoco
// hace del encabezado una fila: la regla convierte en él como en v2.61.0.
func TestTablesRule_Apply_StillBuildsTablesAtTheStartOfARun(t *testing.T) {
	rule := NewTablesRule()
	for _, tc := range []struct{ input, want string }{
		{"# S\n\n| a | b\n| c | d\n| e | f\n", "# S\n\n| a | b |\n|---|---|\n| c | d |\n| e | f |\n\n"},
		{"# S\n\n| ver-nota\n| a | b\n| c | d\n| e | f\n", "# S\n\n| ver-nota\n| a | b |\n|---|---|\n| c | d |\n| e | f |\n\n"},
		{"# S\n\n|---|---|\n| A | B\n| a | b | c\n| d | e | f\n", "# S\n\n|---|---|\n| A | B |   |\n|---|---|---|\n| a | b | c |\n| d | e | f |\n\n"},
		{"# S\n\n|-----|\n| A | B\n| a | b | c\n| d | e | f\n", "# S\n\n|-----|\n| A | B |   |\n|---|---|---|\n| a | b | c |\n| d | e | f |\n\n"},
		{"# S\n\n$$\n|x-y| \\le 1\n|a| = 1\n|b| = 2\n|c| = 3\n$$\n", "# S\n\n$$\n|x-y| \\le 1\n| a | = 1 |\n|---|---|\n| b | = 2 |\n| c | = 3 |\n\n$$\n"},
	} {
		got, err := rule.Apply(tc.input)
		if err != nil {
			t.Fatalf("Apply() error = %v", err)
		}
		if got != tc.want {
			t.Errorf("Apply() =\n%q\nwant\n%q", got, tc.want)
		}
	}
}

// La regla arma una sola tabla por pasada, y la primera candidata termina la
// pasada aunque la deje como está: una lista "clave: valor" o de tabuladores
// que venga después sigue como en v2.61.0, sin convertirse.
func TestTablesRule_Apply_FirstCandidateStillEndsThePass(t *testing.T) {
	rule := NewTablesRule()
	for name, input := range map[string]string{
		"single-cell rows, then a key: value list": "# S\n\n| A | B | C |\n|---|---|---|\n| r0 |\n| r1 |\n| r2 |\n\nNombre: Ana\nEdad: 30\nCiudad: Lima\n",
		"rows without a closing pipe, then a list": "# S\n\n| A | B | C |\n|---|---|---|\n| a | b\n| c | d\n| e | f\n\nNombre: Ana\nEdad: 30\nCiudad: Lima\n",
		"loose single-cell rows, then tabs":        "# S\n\n| r0 |\n| r1 |\n| r2 |\n\nuno\tdos\ttres\ncuatro\tcinco\tseis\nsiete\tocho\tnueve\n",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := rule.Apply(input)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}
			if got != input {
				t.Errorf("Apply() rewrote the document:\n%s\n---\n%s", input, got)
			}
		})
	}
}
