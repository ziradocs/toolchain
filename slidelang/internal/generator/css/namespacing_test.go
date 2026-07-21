// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package css

import (
	"regexp"
	"strings"
	"testing"
)

func TestCSSNamespacing(t *testing.T) {
	// Create CSS builder with default namespacing behavior
	builder := NewCSSBuilder().
		WithRequiredElements([]string{"text", "code"}).
		WithTheme("default")

	// Build CSS
	css := builder.Build()

	// Test basic namespacing
	if !strings.Contains(css, ".slidelang-slide") {
		t.Error("CSS should contain .slidelang-slide class")
	}

	if !strings.Contains(css, ".slidelang-element") {
		t.Error("CSS should contain .slidelang-element class")
	}

	if !strings.Contains(css, ".slidelang-presentation-container") {
		t.Error("CSS should contain .slidelang-presentation-container class")
	}

	// Test that old classes are not present (non-namespaced)
	// Use word boundaries to avoid false positives
	lines := strings.Split(css, "\n")
	for i, line := range lines {
		// Look for .slide that's not part of .slidelang-slide
		if strings.Contains(line, ".slide") && !strings.Contains(line, ".slidelang-slide") {
			// Use regex to check for exact .slide class (not part of another word)
			if matched, _ := regexp.MatchString(`\.slide(?:[^a-zA-Z-]|$)`, line); matched {
				t.Errorf("Found non-namespaced .slide class at line %d: %s", i+1, strings.TrimSpace(line))
			}
		}
		// Look for .element that's not part of .slidelang-element
		if strings.Contains(line, ".element") && !strings.Contains(line, ".slidelang-element") {
			// Use regex to check for exact .element class (not part of another word)
			if matched, _ := regexp.MatchString(`\.element(?:[^a-zA-Z-]|$)`, line); matched {
				t.Errorf("Found non-namespaced .element class at line %d: %s", i+1, strings.TrimSpace(line))
			}
		}
	}

	// Test that embed files are loaded
	if len(css) < 1000 {
		t.Error("CSS seems too short, embed files might not be loading correctly")
	}

	t.Logf("Generated CSS length: %d characters", len(css))
}

func TestCSSBuilderResponsiveToggle(t *testing.T) {
	// Builder includes responsive styles by default
	withResponsive := NewCSSBuilder().
		WithRequiredElements([]string{"text"}).
		WithTheme("default").
		Build()

	if !strings.Contains(withResponsive, "/* === RESPONSIVE STYLES === */") {
		t.Error("expected responsive styles when modular responsive is disabled")
	}

	// When modular responsive is enabled the inline responsive block should disappear
	withoutResponsive := NewCSSBuilder().
		WithRequiredElements([]string{"text"}).
		WithTheme("default").
		WithModularResponsive(true).
		Build()

	if strings.Contains(withoutResponsive, "/* === RESPONSIVE STYLES === */") {
		t.Error("did not expect inline responsive styles when modular responsive is enabled")
	}
}

func TestCSSFileLoader(t *testing.T) {
	loader := NewCSSFileLoader()

	// Test base CSS loading
	baseCSS, err := loader.LoadBaseCSS()
	if err != nil {
		t.Errorf("Failed to load base CSS: %v", err)
	}

	if !strings.Contains(baseCSS, ".slidelang-") {
		t.Error("Base CSS should contain namespaced classes")
	}

	// Test element CSS loading
	elementCSS, err := loader.LoadElementCSS([]string{"text", "code"})
	if err != nil {
		t.Errorf("Failed to load element CSS: %v", err)
	}

	if !strings.Contains(elementCSS, ".slidelang-element") {
		t.Error("Element CSS should contain namespaced element classes")
	}

	t.Logf("Base CSS length: %d, Element CSS length: %d", len(baseCSS), len(elementCSS))
}
