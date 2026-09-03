// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

// DiagramThemeColors agrupa los tokens diagram-* de motor-temas-v2.md §2.2
// para el camino offline/PDF. Zero value = sin tema: reproduce el render
// anterior byte por byte, igual que ChartThemeColors.
//
// Existe porque el rasterizador offline de Mermaid nunca ve el CSS del deck:
// corre Mermaid dentro de Chromium sobre una página temporal propia, y un SVG
// generado por Mermaid no resuelve custom properties. El único canal es
// themeVariables en mermaid.initialize(), que es justo lo que
// MermaidThemeVariables construye.
type DiagramThemeColors struct {
	NodeBG      string // diagram-node-bg
	NodeFG      string // diagram-node-fg
	NodeLine    string // diagram-node-line
	Edge        string // diagram-edge
	EdgeLabelBG string // diagram-edge-label-bg
	AccentBG    string // diagram-accent-bg
	ClusterBG   string // diagram-cluster-bg
	NoteBG      string // diagram-note-bg
	// FontFamily NO es un token diagram-*: viene de font-main, y se incluye
	// acá porque el mapeo del navegador también lo emite (metadata.themeFontMain
	// en buildMermaidThemeVariables). Sin él, el mismo tema daría una fuente
	// en el deck y otra en el PNG/SVG offline.
	FontFamily string
}

// IsZero reporta si no hay ningún color ni fuente de tema puesto.
func (d DiagramThemeColors) IsZero() bool {
	return d == DiagramThemeColors{}
}

// mermaidDefaultFontFamily es el literal que el módulo del navegador emite
// cuando el tema no declara font-main. Se replica para que las dos rutas
// arranquen del mismo default en vez de que offline caiga al de Mermaid.
const mermaidDefaultFontFamily = "arial"

// MermaidThemeVariables traduce los tokens al vocabulario de themeVariables
// de Mermaid. Es un PORT LITERAL de buildMermaidThemeVariables en
// slidelang/internal/generator/template/assets/js/modules/mermaid.js (PR
// #228), que a su vez implementa la tabla "Mapeo a Mermaid" de
// motor-temas-v2.md §2.2 — mismas claves, mismos tokens, mismo default de
// fontFamily.
//
// Que sea un port literal y no una reinterpretación es deliberado: una
// revisión de #250 encontró que portar a Go solo la MITAD del criterio
// equivalente para charts dejó un hueco que el navegador ya no tenía. Si esta
// tabla cambia, tiene que cambiar en los dos lados a la vez.
//
// Devuelve nil con el zero value, para que el caller pueda omitir
// themeVariables por completo y el object literal quede idéntico al de
// siempre.
func (d DiagramThemeColors) MermaidThemeVariables() map[string]string {
	if d.IsZero() {
		return nil
	}
	vars := map[string]string{"fontFamily": mermaidDefaultFontFamily}
	if d.FontFamily != "" {
		vars["fontFamily"] = d.FontFamily
	}
	set := func(value string, keys ...string) {
		if value == "" {
			return
		}
		for _, key := range keys {
			vars[key] = value
		}
	}
	set(d.NodeBG, "mainBkg", "primaryColor")
	set(d.NodeFG, "primaryTextColor", "textColor")
	set(d.NodeLine, "primaryBorderColor", "nodeBorder")
	set(d.Edge, "lineColor")
	set(d.EdgeLabelBG, "edgeLabelBackground")
	set(d.AccentBG, "secondaryColor")
	set(d.ClusterBG, "clusterBkg", "clusterBorder")
	set(d.NoteBG, "noteBkgColor")
	return vars
}

// MermaidExtras devuelve los MermaidExtra que este tema aporta a
// MermaidInitConfigJS, o nil si no hay tema. Se pasa como extra —y no
// concatenado— para que el valor se serialice con encoding/json y ningún
// color pueda romper el object literal, y para que el par de seguridad
// (securityLevel/htmlLabels) siga emitiéndose al final y siga siendo
// inanulable (issue #85).
func (d DiagramThemeColors) MermaidExtras() []MermaidExtra {
	vars := d.MermaidThemeVariables()
	if vars == nil {
		return nil
	}
	return []MermaidExtra{{Key: "themeVariables", Value: vars}}
}
