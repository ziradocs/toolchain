// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"strings"
	"testing"
)

func TestJavaScriptNamespacing(t *testing.T) {
	tb := NewTemplateBuilder()

	// Test JavaScript with querySelector
	jsInput := `
        const slide = document.querySelector('.slide');
        const slides = document.querySelectorAll('.slide');
        const activeSlide = document.querySelector('.slide.active');
        const elements = document.querySelectorAll('[data-element-type]');
    `

	namespacedJS := tb.namespaceJavaScriptSelectors(jsInput)

	// Check that classes are namespaced
	if !strings.Contains(namespacedJS, "querySelector('.slidelang-slide')") {
		t.Error("querySelector('.slide') should be namespaced to querySelector('.slidelang-slide')")
	}

	if !strings.Contains(namespacedJS, "querySelectorAll('.slidelang-slide')") {
		t.Error("querySelectorAll('.slide') should be namespaced to querySelectorAll('.slidelang-slide')")
	}

	// Check that attribute selectors are preserved
	if !strings.Contains(namespacedJS, "querySelectorAll('[data-element-type]')") {
		t.Error("Attribute selectors should be preserved")
	}

	t.Logf("Original JS: %s", jsInput)
	t.Logf("Namespaced JS: %s", namespacedJS)
}

func TestTemplateNamespacing(t *testing.T) {
	tb := NewTemplateBuilder()

	// Test HTML template with classes
	htmlInput := `<div class="slide active"><div class="content"></div></div>`

	namespacedHTML := tb.namespaceTemplateClasses(htmlInput)

	// Check that classes are namespaced
	if !strings.Contains(namespacedHTML, `class="slidelang-slide slidelang-active"`) {
		t.Error("HTML classes should be namespaced")
	}

	if !strings.Contains(namespacedHTML, `class="slidelang-content"`) {
		t.Error("Nested HTML classes should be namespaced")
	}

	t.Logf("Original HTML: %s", htmlInput)
	t.Logf("Namespaced HTML: %s", namespacedHTML)
}

func TestCompleteTemplateGeneration(t *testing.T) {
	tb := NewTemplateBuilder()

	// Test complete HTML body generation
	htmlBody := tb.buildHTMLBody()

	// Check that main containers are namespaced
	if !strings.Contains(htmlBody, `class="slidelang-presentation-container"`) {
		t.Error("Main container should be namespaced")
	}

	if !strings.Contains(htmlBody, `class="slidelang-nav-counter"`) {
		t.Error("Slide counter should be namespaced")
	}

	t.Logf("Generated HTML body contains %d characters", len(htmlBody))
}

func TestJavaScriptGeneration(t *testing.T) {
	tb := NewTemplateBuilder().WithModules([]string{"core"})

	// Test JavaScript generation with namespacing
	js := tb.BuildJS()

	// Check that JavaScript selectors are namespaced
	if !strings.Contains(js, "querySelector('.slidelang-slide") {
		t.Error("JavaScript selectors should be namespaced")
	}

	if !strings.Contains(js, "querySelectorAll('.slidelang-slide") {
		t.Error("JavaScript selector all should be namespaced")
	}

	t.Logf("Generated JavaScript contains %d characters", len(js))
}

// TestUtilitiesJSSurvivesModuleBundling cubre el escenario en el que
// BuildJSWithModules empaqueta "utilities" directamente en presentation.js
// (cuando es el único módulo requerido) en vez de generarse como archivo
// externo aparte: en ese camino, namespaceJavaScriptSelectors SÍ procesa el
// código de utilities.go, y un selector escrito como bare (p.ej. '.tab') se
// reescribiría de vuelta a '.slidelang-tab' — si el HTML real emitiera la
// clase sin prefijo, el selector post-namespacing jamás la encontraría
// (issue #115). Verificar que utilities.go usa selectores ya prefijados
// ('.slidelang-tab', '.slidelang-code-block', '.slidelang-copy-button'),
// que namespaceJavaScriptSelectors deja intactos por ser idempotente sobre
// strings que ya empiezan con "slidelang-".
func TestUtilitiesJSSurvivesModuleBundling(t *testing.T) {
	tb := NewTemplateBuilder()
	js := tb.BuildJSWithModules([]string{"utilities"})

	if strings.Contains(js, "querySelectorAll('.tab')") || strings.Contains(js, "querySelectorAll('.code-block')") {
		t.Errorf("bundled utilities JS contains un-namespaced bare tab/code-block selectors")
	}
	if !strings.Contains(js, "querySelectorAll('.slidelang-tab')") {
		t.Error("expected querySelectorAll('.slidelang-tab') in bundled utilities JS")
	}
	if !strings.Contains(js, "querySelectorAll('.slidelang-code-block')") {
		t.Error("expected querySelectorAll('.slidelang-code-block') in bundled utilities JS")
	}
	if !strings.Contains(js, "querySelector('.slidelang-copy-button')") {
		t.Error("expected querySelector('.slidelang-copy-button') duplicate-check in bundled utilities JS")
	}
	if strings.Contains(js, "className = 'copy-button'") {
		t.Error("copy button className must carry the slidelang- prefix so the duplicate-check can match it")
	}
}
