// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/json"
	"strings"
)

// DiagramThemeColors is the renderer-neutral, resolved visual contract for
// diagram engines. Callers pass literal colors (never CSS var() references):
// Mermaid produces self-contained SVGs and PlantUML receives source text, so
// neither can resolve a document's custom properties after the fact.
//
// The zero value deliberately preserves each engine's historical defaults.
// Color values are checked before they reach an engine: an invalid custom
// theme must degrade to that engine's defaults, not make a whole diagram fail.
type DiagramThemeColors struct {
	FontFamily       string
	NodeBackground   string
	NodeForeground   string
	NodeBorder       string
	Edge             string
	EdgeLabelBack    string
	ClusterBack      string
	NoteBack         string
	AccentBackground string
}

// IsZero reports whether no visual override was supplied.
func (t DiagramThemeColors) IsZero() bool {
	return t.FontFamily == "" && t.NodeBackground == "" && t.NodeForeground == "" &&
		t.NodeBorder == "" && t.Edge == "" && t.EdgeLabelBack == "" &&
		t.ClusterBack == "" && t.NoteBack == "" && t.AccentBackground == ""
}

// MermaidThemeVariables returns only accepted Mermaid themeVariables. The
// object is later JSON-encoded before entering a script, so a font family
// cannot alter the surrounding JavaScript.
func (t DiagramThemeColors) MermaidThemeVariables() map[string]string {
	vars := make(map[string]string)
	if t.FontFamily != "" {
		vars["fontFamily"] = t.FontFamily
	}
	put := func(color string, keys ...string) {
		if !validDiagramColor(color) {
			return
		}
		for _, key := range keys {
			vars[key] = color
		}
	}
	put(t.NodeBackground, "mainBkg", "primaryColor")
	put(t.NodeForeground, "primaryTextColor", "textColor")
	put(t.NodeBorder, "primaryBorderColor", "nodeBorder")
	put(t.Edge, "lineColor")
	put(t.EdgeLabelBack, "edgeLabelBackground")
	put(t.AccentBackground, "secondaryColor")
	put(t.ClusterBack, "clusterBkg", "clusterBorder")
	put(t.NoteBack, "noteBkgColor")
	if len(vars) == 0 {
		return nil
	}
	return vars
}

// CacheFingerprint is stable and includes every effective Mermaid input.
// Offline fetchers use it so reusing an output directory with another theme
// cannot return the first theme's cached SVG.
func (t DiagramThemeColors) CacheFingerprint() string {
	data, err := json.Marshal(t.MermaidThemeVariables())
	if err != nil {
		return ""
	}
	return string(data)
}

// ApplyMermaidTheme prepends Mermaid's documented init directive for engines
// we do not control directly (currently the opt-in Kroki path used by PPTX).
// A directive authored in the diagram itself remains later in the source and
// therefore can deliberately override this presentation default.
func ApplyMermaidTheme(content string, theme DiagramThemeColors) string {
	variables := theme.MermaidThemeVariables()
	if len(variables) == 0 {
		return content
	}
	config, err := json.Marshal(map[string]any{
		"theme":          "base",
		"themeVariables": variables,
	})
	if err != nil {
		return content
	}
	return "%%{init: " + string(config) + "}%%\n" + content
}

// ApplyPlantUMLTheme adds skinparams immediately after @startuml. Author
// skinparams occur later in the source and retain precedence under PlantUML's
// normal last-value-wins behavior. The same transformed source is used by the
// browser URL, PlantUML server and Kroki paths.
func ApplyPlantUMLTheme(content string, theme DiagramThemeColors) string {
	var lines []string
	put := func(name, color string) {
		if validDiagramColor(color) {
			lines = append(lines, "skinparam "+name+" "+color)
		}
	}
	if theme.FontFamily != "" && !strings.ContainsAny(theme.FontFamily, "\r\n") {
		lines = append(lines, "skinparam defaultFontName "+theme.FontFamily)
	}
	put("backgroundColor", theme.NodeBackground)
	put("defaultFontColor", theme.NodeForeground)
	put("ArrowColor", theme.Edge)
	put("ArrowFontColor", theme.NodeForeground)
	put("ClassBackgroundColor", theme.NodeBackground)
	put("ClassBorderColor", theme.NodeBorder)
	put("ClassFontColor", theme.NodeForeground)
	put("NoteBackgroundColor", theme.NoteBack)
	put("NoteBorderColor", theme.NodeBorder)
	put("PackageBackgroundColor", theme.ClusterBack)
	put("PackageBorderColor", theme.NodeBorder)
	put("RectangleBackgroundColor", theme.AccentBackground)
	put("RectangleBorderColor", theme.NodeBorder)
	if len(lines) == 0 {
		return content
	}

	start := strings.Index(strings.ToLower(content), "@startuml")
	if start < 0 {
		return content
	}
	newline := strings.IndexByte(content[start:], '\n')
	if newline < 0 {
		return content + "\n" + strings.Join(lines, "\n")
	}
	insertAt := start + newline + 1
	return content[:insertAt] + strings.Join(lines, "\n") + "\n" + content[insertAt:]
}

func validDiagramColor(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	// SanitizeColor returns its fallback for unsafe/unrecognised values. Do
	// not silently turn a broken theme token into blue; leave that engine's
	// own default intact instead.
	return strings.EqualFold(SanitizeColor(value), value)
}
