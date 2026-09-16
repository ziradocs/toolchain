// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import "go.ziradocs.com/core/v2/renderer"

// resolveDiagramThemeColors maps DocLang's theme variables to the shared
// diagram contract. Dedicated --doclang-diagram-* variables take precedence;
// the document's established page/text/link variables provide sensible theme
// defaults without requiring every existing theme to be rewritten.
func resolveDiagramThemeColors(vars map[string]string) renderer.DiagramThemeColors {
	value := func(name, fallback string) string {
		if v := vars["--doclang-"+name]; v != "" {
			return v
		}
		return vars["--doclang-"+fallback]
	}
	return renderer.DiagramThemeColors{
		FontFamily:       vars["--doclang-font-main"],
		NodeBackground:   value("diagram-node-bg", "page-bg"),
		NodeForeground:   value("diagram-node-fg", "text-color"),
		NodeBorder:       value("diagram-node-line", "table-border"),
		Edge:             value("diagram-edge", "text-light"),
		EdgeLabelBack:    value("diagram-edge-label-bg", "page-bg"),
		ClusterBack:      value("diagram-cluster-bg", "quote-bg"),
		NoteBack:         value("diagram-note-bg", "alert-info-bg"),
		AccentBackground: value("diagram-accent-bg", "link-color"),
	}
}
