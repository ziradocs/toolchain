// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"strings"
	"testing"
)

func TestNavigationJavaScriptNamespacing(t *testing.T) {
	builder := NewTemplateBuilder()

	// Generate navigation JS through the builder (which applies namespacing)
	js := builder.BuildJSWithModules([]string{"navigation"})

	// Verify that querySelector calls are properly namespaced
	testCases := []struct {
		namespaced string
		desc       string
	}{
		{
			namespaced: "querySelectorAll('.slidelang-slide')",
			desc:       "slide selector should be namespaced",
		},
		{
			namespaced: "querySelector('.slidelang-progress-bar')",
			desc:       "progress-bar selector should be namespaced",
		},
		{
			namespaced: "querySelector('.slidelang-nav-counter')",
			desc:       "nav-counter selector should be namespaced",
		},
	}

	for _, tc := range testCases {
		// Verify the namespaced selector is present
		if !strings.Contains(js, tc.namespaced) {
			t.Errorf("Expected namespaced selector %s not found in generated JS", tc.namespaced)
		}
	}

	// Verify non-namespaced selectors are not present (should all be namespaced)
	nonNamespacedPatterns := []string{
		"querySelector('.slide')",
		"querySelector('.progress-bar')",
		"querySelector('.nav-counter')",
		"querySelector('.floating-menu')",
	}

	for _, pattern := range nonNamespacedPatterns {
		if strings.Contains(js, pattern) {
			t.Errorf("Found non-namespaced selector %s in generated JS", pattern)
		}
	}

	// Verify dynamically created HTML classes are namespaced
	dynamicClassTests := []string{
		"slidelang-floating-menu",
		"slidelang-advanced-menu",
		"slidelang-menu-btn",
		"slidelang-visible",
		"slidelang-presentation-mode",
	}

	for _, className := range dynamicClassTests {
		if !strings.Contains(js, className) {
			t.Errorf("Expected dynamic class name %s not found in navigation JS", className)
		}
	}

	t.Logf("✅ Navigation JavaScript namespacing test passed - generated %d characters", len(js))
}

func TestNavigationHTMLElementsNamespacing(t *testing.T) {
	// Test that the navigation JS creates properly namespaced HTML
	navJS := GetNavigationJS()

	// This JS will be processed by namespaceJavaScriptSelectors later,
	// but we should verify that dynamically created HTML classes are already namespaced
	htmlClassTests := []string{
		`class="slidelang-floating-menu"`,
		`class="slidelang-advanced-menu"`,
		`class="slidelang-menu-btn"`,
		`className = 'slidelang-progress-bar'`,
	}

	for _, htmlClass := range htmlClassTests {
		if !strings.Contains(navJS, htmlClass) {
			t.Errorf("Expected HTML class %s not found in navigation JS", htmlClass)
		}
	}

	// Verify old non-namespaced classes are not present in dynamic assignments
	oldClassTests := []string{
		`class="floating-menu"`,
		`class="advanced-menu"`,
		`class="menu-btn"`,
		`className = 'progress-bar'`,
	}

	for _, oldClass := range oldClassTests {
		if strings.Contains(navJS, oldClass) {
			t.Errorf("Found non-namespaced HTML class %s in navigation JS", oldClass)
		}
	}

	t.Logf("✅ Navigation HTML elements namespacing test passed")
}
