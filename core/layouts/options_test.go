// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package layouts

import (
	"strings"
	"testing"
)

func TestApply_ValidValues(t *testing.T) {
	var cfg Config
	if err := Apply(&cfg, "comparison", "columns", "3"); err != nil {
		t.Fatalf("columns=3: %v", err)
	}
	if cfg.Columns != 3 {
		t.Errorf("Columns = %d", cfg.Columns)
	}

	if err := Apply(&cfg, "hero", "align", "LEFT"); err != nil {
		t.Fatalf("align=LEFT: %v", err)
	}
	if cfg.Align != "left" {
		t.Errorf("Align = %q — el valor debe normalizarse a minúsculas", cfg.Align)
	}
}

// El punto entero de tipar las opciones (issue #255): un typo tiene que ser
// distinguible de una llave que algún renderer podría usar.
func TestApply_RejectsAndExplains(t *testing.T) {
	for _, tc := range []struct {
		name, layout, key, value string
		wantIn                   string
	}{
		{"typo", "comparison", "colums", "2", "not a layout option"},
		{"opción de otro layout", "hero", "columns", "2", `not an option of layout "hero"`},
		{"fuera de rango", "comparison", "columns", "9", "between 1 and 4"},
		{"no es número", "comparison", "columns", "dos", "whole number"},
		{"enum inválido", "hero", "align", "justify", "left/center"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cfg Config
			err := Apply(&cfg, tc.layout, tc.key, tc.value)
			if err == nil {
				t.Fatalf("se esperaba error para %s=%s en %s", tc.key, tc.value, tc.layout)
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("mensaje %q no contiene %q", err.Error(), tc.wantIn)
			}
			if !cfg.IsZero() {
				t.Errorf("un valor rechazado no debe escribir nada: %+v", cfg)
			}
		})
	}
}

// El mensaje de "opción de otro layout" tiene que decir cuáles SÍ acepta:
// sin eso, el autor sabe que se equivocó pero no dónde corregirlo.
func TestApply_WrongLayoutMessageListsTheAcceptedOnes(t *testing.T) {
	var cfg Config
	err := Apply(&cfg, "hero", "columns", "2")
	if err == nil {
		t.Fatal("se esperaba error")
	}
	if !strings.Contains(err.Error(), "align") {
		t.Errorf("el mensaje no dice qué acepta hero: %q", err.Error())
	}
}

func TestAcceptsAndIsKnownOption(t *testing.T) {
	if !Accepts("comparison", "columns") {
		t.Error("comparison debería aceptar columns")
	}
	if Accepts("hero", "columns") {
		t.Error("hero no debería aceptar columns")
	}
	if !IsKnownOption("columns") || !IsKnownOption("align") {
		t.Error("columns y align son opciones conocidas")
	}
	if IsKnownOption("colums") {
		t.Error("un typo no es una opción conocida")
	}
}

// Cada layout del registro tiene que ser un tipo de slide real, o sus opciones
// nunca se podrían escribir.
func TestRegistryLayoutsAreRealSlideTypes(t *testing.T) {
	// La lista se repite acá en vez de importar core/linter: este paquete no
	// importa nada del toolchain a propósito (lo consumen los dos parsers y el
	// linter, y una dependencia al revés sería un ciclo). Si un layout deja de
	// existir, LAYOUT_UNKNOWN lo reporta del otro lado.
	known := map[string]bool{
		"title": true, "title_slide": true, "content": true, "section": true,
		"comparison": true, "stats": true, "code_example": true, "hero": true,
		"testimonial": true, "timeline": true, "before_after": true, "pricing": true,
		"team": true, "feature_showcase": true, "call_to_action": true,
		"dashboard": true, "process": true, "default": true, "closing": true,
	}
	for _, layout := range LayoutsWithOptions() {
		if !known[layout] {
			t.Errorf("el registro declara opciones para %q, que no es un tipo de slide con schema", layout)
		}
	}
}

// Options devuelve una copia: mutarla no puede corromper el registro.
func TestOptionsReturnsACopy(t *testing.T) {
	specs := Options("comparison")
	if len(specs) == 0 {
		t.Fatal("comparison debería declarar opciones")
	}
	specs[0].Max = 99

	var cfg Config
	if err := Apply(&cfg, "comparison", "columns", "9"); err == nil {
		t.Error("mutar la copia devuelta cambió el registro")
	}
}
