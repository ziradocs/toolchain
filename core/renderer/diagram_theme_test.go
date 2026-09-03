// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"
	"strings"
	"testing"
)

func fullDiagramTheme() DiagramThemeColors {
	return DiagramThemeColors{
		NodeBG:      "#nodebg",
		NodeFG:      "#nodefg",
		NodeLine:    "#nodeline",
		Edge:        "#edge",
		EdgeLabelBG: "#edgelabelbg",
		AccentBG:    "#accentbg",
		ClusterBG:   "#clusterbg",
		NoteBG:      "#notebg",
		FontFamily:  "Inter",
	}
}

// TestMermaidThemeVariables_MatchesBrowserMapping fija el mapeo TOKEN→CLAVE
// exacto que buildMermaidThemeVariables emite en el navegador
// (slidelang/.../modules/mermaid.js, PR #228). La tabla se duplica acá a
// propósito, escrita a mano: si alguien cambia una clave de un lado, este
// test falla y obliga a cambiar el otro.
//
// Una revisión de #250 encontró que portar a Go solo la MITAD de un criterio
// equivalente para charts dejó un hueco que el navegador ya no tenía; este
// test existe para que ese modo de falla no se repita en silencio.
func TestMermaidThemeVariables_MatchesBrowserMapping(t *testing.T) {
	vars := fullDiagramTheme().MermaidThemeVariables()

	want := map[string]string{
		"mainBkg":             "#nodebg",
		"primaryColor":        "#nodebg",
		"primaryTextColor":    "#nodefg",
		"textColor":           "#nodefg",
		"primaryBorderColor":  "#nodeline",
		"nodeBorder":          "#nodeline",
		"lineColor":           "#edge",
		"edgeLabelBackground": "#edgelabelbg",
		"secondaryColor":      "#accentbg",
		"clusterBkg":          "#clusterbg",
		"clusterBorder":       "#clusterbg",
		"noteBkgColor":        "#notebg",
		"fontFamily":          "Inter",
	}
	if len(vars) != len(want) {
		t.Errorf("el mapeo emitió %d claves, esperaba %d: %#v", len(vars), len(want), vars)
	}
	for key, expected := range want {
		if vars[key] != expected {
			t.Errorf("themeVariables[%q] = %q, want %q", key, vars[key], expected)
		}
	}
}

// TestMermaidThemeVariables_ZeroValueEmitsNothing es la garantía de "sin
// tema, byte por byte igual": nil deja que el caller omita themeVariables
// por completo.
func TestMermaidThemeVariables_ZeroValueEmitsNothing(t *testing.T) {
	if vars := (DiagramThemeColors{}).MermaidThemeVariables(); vars != nil {
		t.Errorf("el zero value debe emitir nil, got %#v", vars)
	}
	if extras := (DiagramThemeColors{}).MermaidExtras(); extras != nil {
		t.Errorf("el zero value no debe aportar extras, got %#v", extras)
	}
	if got := MermaidInitConfigJS(true, (DiagramThemeColors{}).MermaidExtras()...); got != MermaidInitConfigJS(true) {
		t.Errorf("sin tema el object literal debe ser idéntico al de siempre:\n got %s\nwant %s", got, MermaidInitConfigJS(true))
	}
}

// TestMermaidThemeVariables_PartialTokensAndFontDefault comprueba que cada
// token es independiente y que la fuente cae al mismo literal que usa el
// navegador cuando el tema no declara font-main.
func TestMermaidThemeVariables_PartialTokensAndFontDefault(t *testing.T) {
	vars := DiagramThemeColors{Edge: "#solo-edge"}.MermaidThemeVariables()

	if vars["lineColor"] != "#solo-edge" {
		t.Errorf("lineColor = %q, want #solo-edge", vars["lineColor"])
	}
	if vars["fontFamily"] != "arial" {
		t.Errorf("fontFamily = %q, want arial (el mismo default del navegador)", vars["fontFamily"])
	}
	for _, ausente := range []string{"mainBkg", "primaryColor", "noteBkgColor", "clusterBkg"} {
		if _, existe := vars[ausente]; existe {
			t.Errorf("un token no declarado no debe emitir %q: %#v", ausente, vars)
		}
	}
}

// TestMermaidInitConfigJS_ThemeCannotBreakTheSecurityPair fija que el tema
// entra como MermaidExtra (serializado con encoding/json) y por lo tanto no
// puede ni romper el object literal ni anular securityLevel/htmlLabels, que
// se emiten al final — la invariante estructural del issue #85.
func TestMermaidInitConfigJS_ThemeCannotBreakTheSecurityPair(t *testing.T) {
	hostil := DiagramThemeColors{NodeBG: `"} ); alert(1); //`, Edge: "</script>"}
	got := MermaidInitConfigJS(true, hostil.MermaidExtras()...)

	if !strings.HasSuffix(got, "securityLevel: 'strict', htmlLabels: false }") {
		t.Errorf("el par de seguridad debe seguir cerrando el literal: %s", got)
	}
	if !strings.Contains(got, "themeVariables:") {
		t.Errorf("el tema no llegó al literal: %s", got)
	}

	// La comilla del payload tiene que salir ESCAPADA (\"), que es lo que la
	// deja como texto dentro del string JSON en vez de cerrarlo. Buscar la
	// subcadena `); alert(1)` a secas no sirve: aparece legítimamente dentro
	// del string ya escapado, así que un test así falla sobre código correcto
	// — es el error que tenía la primera versión de esta aserción.
	if !strings.Contains(got, `\"} ); alert(1); //`) {
		t.Errorf("la comilla del payload no quedó escapada: %s", got)
	}
	// Y lo que de verdad importa para una página HTML: nada puede cerrar el
	// <script> que envuelve esta llamada. encoding/json escapa < y > como
	// </> justamente por esto.
	if strings.Contains(got, "</script>") {
		t.Errorf("el valor pudo cerrar el bloque <script>: %s", got)
	}
	// La forma esperada se calcula con el mismo encoder en vez de
	// hardcodear la secuencia de escape: así el test dice "sale exactamente
	// como encoding/json lo escaparía" y no depende de cómo se escriba
	// < en el fuente del test.
	escaped, err := json.Marshal("</script>")
	if err != nil {
		t.Fatalf("marshal de referencia: %v", err)
	}
	if !strings.Contains(got, strings.Trim(string(escaped), `"`)) {
		t.Errorf("esperaba el cierre de script en su forma escapada (%s): %s", escaped, got)
	}
}
