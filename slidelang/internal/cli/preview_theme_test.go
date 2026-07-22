// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/slidelang/v2/internal/generator"
)

// newTestPreviewServer builds a minimal PreviewServer suitable for exercising
// generatePresentation directly, without starting the HTTP server.
func newTestPreviewServer(t *testing.T) *PreviewServer {
	t.Helper()
	log := util.NewConsoleLogger(util.LevelError, false)
	return &PreviewServer{
		ThemePath: "unused-theme-path",
		logger:    log,
		parser:    parser.New(log),
		generator: generator.New(log),
	}
}

// TestGeneratePresentation_OversizedInput_RejectedBeforeParsing cubre issue
// #69: generatePresentation es un segundo punto de entrada al parser (usado
// por el servidor `slidelang themes preview`, de larga duración) que no
// tenía el guard de tamaño/timeout/recover que sí protege a `build` (ver
// util.CheckInputSize/util.RunGuarded en build.go, issue #22/#65). Un
// --sample apuntando a un archivo desmedido debía colgar o tumbar el
// proceso completo del servidor en vez de solo una invocación de build.
//
// Este test confirma que una entrada mayor al límite configurado es
// rechazada por el cap de tamaño ANTES de llegar a parser.Parse, y que
// generatePresentation retorna una página de error en vez de propagar un
// panic o colgarse.
func TestGeneratePresentation_OversizedInput_RejectedBeforeParsing(t *testing.T) {
	ps := newTestPreviewServer(t)

	// Construir un input que exceda util.DefaultMaxInputBytes (10 MB) sin
	// depender de --max-size ni de la variable de entorno.
	oversized := strings.Repeat("a", util.DefaultMaxInputBytes+1)

	html := ps.generatePresentation(oversized)

	if !strings.Contains(html, "Preview aborted") {
		t.Fatalf("expected the oversized input to be rejected with a guard error page, got:\n%s", html)
	}
	if !strings.Contains(html, "too large") {
		t.Errorf("expected the guard error page to mention the size-cap error, got:\n%s", html)
	}
}

// TestGeneratePresentation_NormalInput_StillParses es una prueba de
// sanidad: el guard añadido no debe romper el camino feliz para una
// entrada normal dentro del límite.
func TestGeneratePresentation_NormalInput_StillParses(t *testing.T) {
	ps := newTestPreviewServer(t)

	slides := `---
mode: flex
title: "Guard Sanity Check"
---

# Hello

Some sample content.
`

	html := ps.generatePresentation(slides)

	if strings.Contains(html, "Preview aborted") {
		t.Fatalf("expected normal input to parse without tripping the guard, got:\n%s", html)
	}
}
