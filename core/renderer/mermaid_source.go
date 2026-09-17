// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"regexp"
	"strings"
)

// PrepareMermaidContent applies display-only compatibility fixes. The parser
// deliberately does not call this: Mermaid source is opaque AST data and the
// formatter must round-trip it without adding a header, flattening whitespace,
// or rewriting labels (issue #346).
func PrepareMermaidContent(content, diagramType string) string {
	if diagramType != "flowchart" {
		return content
	}
	lines := strings.Split(content, "\n")
	for i := range lines {
		lines[i] = fixMermaidFlowchartLine(lines[i])
	}
	return strings.Join(lines, "\n")
}

var (
	mermaidNumberedNode    = regexp.MustCompile(`([A-Z][A-Z0-9]*)\[(\d+\.\s+[^\]]+)\]`)
	mermaidNumberedCircle  = regexp.MustCompile(`([A-Z][A-Z0-9]*)\(\((\d+\.\s+[^\)]+)\)\)`)
	mermaidNumberedDiamond = regexp.MustCompile(`([A-Z][A-Z0-9]*)\{(\d+\.\s+[^\}]+)\}`)
	mermaidNumberedParen   = regexp.MustCompile(`([A-Z][A-Z0-9]*)\((\d+\.\s+[^\)]+)\)`)
)

func fixMermaidFlowchartLine(line string) string {
	replace := func(re *regexp.Regexp, open, close string) (string, bool) {
		if !re.MatchString(line) {
			return line, false
		}
		return re.ReplaceAllStringFunc(line, func(match string) string {
			parts := re.FindStringSubmatch(match)
			return parts[1] + open + parts[2] + close
		}), true
	}
	if out, ok := replace(mermaidNumberedNode, "['", "']"); ok {
		return out
	}
	if out, ok := replace(mermaidNumberedCircle, "(('", "'))"); ok {
		return out
	}
	if out, ok := replace(mermaidNumberedDiamond, "{'", "'}"); ok {
		return out
	}
	if out, ok := replace(mermaidNumberedParen, "('", "')"); ok {
		return out
	}
	return line
}
