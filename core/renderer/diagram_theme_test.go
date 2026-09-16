// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"strings"
	"testing"
)

func TestMermaidInitConfigJSWithTheme_MapsVisualContract(t *testing.T) {
	config := MermaidInitConfigJSWithTheme(false, DiagramThemeColors{
		FontFamily:     "Inter, sans-serif",
		NodeBackground: "#102030",
		NodeForeground: "#f0f0f0",
		Edge:           "#789abc",
	})

	for _, want := range []string{
		`"fontFamily":"Inter, sans-serif"`,
		`"mainBkg":"#102030"`,
		`"primaryTextColor":"#f0f0f0"`,
		`"lineColor":"#789abc"`,
		"securityLevel: 'strict'",
	} {
		if !strings.Contains(config, want) {
			t.Errorf("themed Mermaid config missing %q: %s", want, config)
		}
	}
}

func TestApplyPlantUMLTheme_PreservesAuthorOverrideOrder(t *testing.T) {
	content := "@startuml\nskinparam ArrowColor #author\nAlice -> Bob\n@enduml"
	got := ApplyPlantUMLTheme(content, DiagramThemeColors{Edge: "#123456"})
	wantPrefix := "@startuml\nskinparam ArrowColor #123456\nskinparam ArrowColor #author"
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("theme must follow @startuml and precede author skinparams:\n%s", got)
	}
}

func TestApplyMermaidTheme_IgnoresUnsafeColors(t *testing.T) {
	content := "graph TD\nA-->B"
	got := ApplyMermaidTheme(content, DiagramThemeColors{NodeBackground: "red; alert(1)"})
	if got != content {
		t.Fatalf("unsafe diagram color must not create an init directive: %q", got)
	}
}
