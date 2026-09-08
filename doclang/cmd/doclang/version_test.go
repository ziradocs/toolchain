// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

// Un binario instalado con `go install …@latest` es un release, pero
// goreleaser no lo tocó, así que `version` sigue en "dev" y `--version`
// respondía "dev". Medido sobre el binario publicado de v2.32.4:
// `go version -m` leía v2.32.4 y `--version` decía dev.
//
// El fallback lee la versión del módulo del propio binario. Los dos casos que
// NO tiene que tocar son los que este test fija: un valor estampado gana
// siempre, y "(devel)" —lo que devuelve un `go build` local— no aporta nada
// sobre el fallback y se queda en "dev".
func TestResolveVersion_PrefersTheStampedValue(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })

	version = "v9.9.9"
	if got := resolveVersion(); got != "v9.9.9" {
		t.Errorf("resolveVersion() = %q, se esperaba el valor estampado %q", got, "v9.9.9")
	}
}

// Con `version` en "dev", el resultado sale de debug.ReadBuildInfo. Dentro de
// `go test` el binario de prueba reporta "(devel)" o vacío, así que lo que se
// puede afirmar sin depender del entorno es que NUNCA devuelve "(devel)": o
// hay una versión de módulo de verdad, o se queda en "dev".
func TestResolveVersion_NeverReportsDevel(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })

	version = "dev"
	got := resolveVersion()
	if got == "(devel)" {
		t.Errorf("resolveVersion() = %q: ese valor no se le muestra a nadie", got)
	}
	if got == "" {
		t.Error("resolveVersion() devolvió vacío")
	}
}
