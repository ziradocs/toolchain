// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package layouts

import (
	"strconv"
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

// Toda opción del registro tiene que tener un campo donde escribirse.
//
// Apply valida por spec.Type pero asigna con un `switch key`. Una opción nueva
// en el registro sin su rama devolvía `err == nil` sin haber escrito nada:
// Config seguía en cero, el parser no colgaba ningún LayoutConfig, y la
// opción se perdía sin diagnóstico — un validador que reporta éxito habiendo
// tirado el valor, en el paquete escrito para que eso no pase. El test
// recorre el registro entero, así que agregar `rows` sin la rama falla acá y
// no en el render de alguien.
func TestApply_EveryRegisteredOptionReachesConfig(t *testing.T) {
	for _, layout := range LayoutsWithOptions() {
		for _, spec := range Options(layout) {
			t.Run(layout+"/"+spec.Name, func(t *testing.T) {
				value := ""
				switch spec.Type {
				case OptionInt:
					value = strconv.Itoa(spec.Min)
				case OptionEnum:
					if len(spec.Values) == 0 {
						t.Fatalf("la opción enum %q no declara valores", spec.Name)
					}
					value = spec.Values[0]
				}

				var cfg Config
				if err := Apply(&cfg, layout, spec.Name, value); err != nil {
					t.Fatalf("Apply(%s, %s=%s) devolvió error: %v", layout, spec.Name, value, err)
				}
				if cfg.IsZero() {
					t.Errorf("Apply(%s, %s=%s) devolvió nil sin escribir nada en Config: "+
						"la opción está en el registro pero Apply no tiene rama que la asigne",
						layout, spec.Name, value)
				}
			})
		}
	}
}

// La copia que devuelve Options tiene que ser completa: `Values` es un slice,
// y devolver un alias del arreglo del registro deja que un llamador lo
// corrompa para todos. TestOptionsReturnsACopy solo mutaba `Max`, un campo de
// valor, así que pasaba igual.
func TestOptionsReturnsADeepCopy(t *testing.T) {
	specs := Options("hero")
	var target *OptionSpec
	for i := range specs {
		if len(specs[i].Values) > 0 {
			target = &specs[i]
			break
		}
	}
	if target == nil {
		t.Skip("ninguna opción de `hero` declara Values")
	}

	original := target.Values[0]
	target.Values[0] = "mutado"

	if again := Options("hero"); again[0].Values[0] != original {
		t.Errorf("mutar la copia cambió el registro: Values[0] = %q, se esperaba %q",
			again[0].Values[0], original)
	}
}
