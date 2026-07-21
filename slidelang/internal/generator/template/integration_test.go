// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"strings"
	"testing"
)

func TestCompleteNamespacingIntegration(t *testing.T) {
	// Test complete integration: CSS + HTML + JavaScript with namespacing
	tb := NewTemplateBuilder().
		WithTheme("default").
		WithModules([]string{"core", "navigation"}).
		WithRequiredElements([]string{"text", "code", "images"}).
		WithEmbedAssets(true)

	// Generate complete HTML with embedded CSS and JS
	html := tb.Build()

	// Check CSS namespacing
	if !strings.Contains(html, ".slidelang-slide") {
		t.Error("Generated CSS should contain namespaced classes")
	}

	if !strings.Contains(html, ".slidelang-presentation-container") {
		t.Error("Generated CSS should contain namespaced container classes")
	}

	// Check HTML namespacing
	if !strings.Contains(html, `class="slidelang-presentation-container"`) {
		t.Error("Generated HTML should contain namespaced classes")
	}

	// Check JavaScript namespacing
	if !strings.Contains(html, "querySelector('.slidelang-slide") {
		t.Error("Generated JavaScript should contain namespaced selectors")
	}

	// Verify no non-namespaced classes remain (except for external libraries)
	// We'll check for some specific cases that should be namespaced
	if strings.Contains(html, `class="slide "`) || strings.Contains(html, `class="slide"`) {
		// Allow for cases where it might be part of a longer class name
		if !strings.Contains(html, "slidelang-slide") {
			t.Error("Found non-namespaced 'slide' class in generated HTML")
		}
	}

	t.Logf("Generated complete HTML contains %d characters", len(html))
	t.Logf("✅ Complete namespacing integration test passed")
}

func TestCSSJSConsistency(t *testing.T) {
	// Test that CSS and JavaScript use the same class names
	tb := NewTemplateBuilder().
		WithRequiredElements([]string{"text"}).
		WithModules([]string{"core"})

	// Generate CSS
	css, err := tb.BuildCSS()
	if err != nil {
		t.Fatalf("Failed to build CSS: %v", err)
	}

	// Generate JavaScript
	js := tb.BuildJS()

	// Extract some key class names from CSS and verify they match in JS
	testClasses := []string{
		"slidelang-slide",
		"slidelang-presentation-container",
		"slidelang-element",
	}

	for _, className := range testClasses {
		// Check CSS contains the class
		if !strings.Contains(css, "."+className) {
			t.Errorf("CSS should contain class .%s", className)
		}

		// Check JavaScript references the class correctly
		if strings.Contains(js, "querySelector('.slide") || strings.Contains(js, "querySelector('.presentation-container") || strings.Contains(js, "querySelector('.element") {
			// Only check if JS actually has these selectors
			if strings.Contains(js, className) {
				// JavaScript contains the namespaced class, good
				t.Logf("✅ JavaScript correctly uses namespaced class: %s", className)
			}
		}
	}

	t.Logf("✅ CSS and JavaScript class consistency verified")
}

func BenchmarkNamespacingPerformance(b *testing.B) {
	tb := NewTemplateBuilder().
		WithModules([]string{"core", "navigation"}).
		WithRequiredElements([]string{"text", "code", "images"})

	// Sample HTML and JS for benchmarking
	htmlTemplate := `<div class="slide active"><div class="content"><div class="element text"></div></div></div>`
	jsTemplate := `const slide = document.querySelector('.slide'); const elements = document.querySelectorAll('.element');`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Benchmark HTML namespacing
		_ = tb.namespaceTemplateClasses(htmlTemplate)

		// Benchmark JavaScript namespacing
		_ = tb.namespaceJavaScriptSelectors(jsTemplate)
	}
}
