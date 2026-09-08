// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package main

import "testing"

// Un binario instalado con `go install …@latest` es un release, pero goreleaser
// no lo tocó, así que `version` sigue en "dev" y `--version` respondía "dev".
// Medido sobre el binario publicado de v2.32.4: `go version -m` leía v2.32.4 y
// `--version` decía dev.
//
// La tabla prueba pickVersion, que es pura. La primera versión de este test
// llamaba a resolveVersion() y por eso no probaba nada: adentro de `go test`,
// ReadBuildInfo describe al binario de PRUEBA, así que el resultado dependía
// del harness y no del arreglo — mutar el fallback para que descartara siempre
// la build info dejaba los cuatro tests en verde.
func TestPickVersion(t *testing.T) {
	for _, tc := range []struct {
		name          string
		stamped       string
		fromBuildInfo string
		want          string
	}{
		{"instalado con @latest", "dev", "v2.32.5", "v2.32.5"},
		{"go build local", "dev", "(devel)", "dev"},
		{"sin build info", "dev", "", "dev"},
		{"estampado por goreleaser", "v2.32.5", "", "v2.32.5"},
		{"el estampado le gana a la build info", "v2.32.5", "v9.9.9", "v2.32.5"},
		// Un `go install` de un commit sin tag da una pseudo-versión. Es
		// fea pero es MÁS informativa que "dev": identifica el commit.
		{"pseudo-versión", "dev", "v0.0.0-20260908120000-abcdef123456", "v0.0.0-20260908120000-abcdef123456"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := pickVersion(tc.stamped, tc.fromBuildInfo); got != tc.want {
				t.Errorf("pickVersion(%q, %q) = %q, se esperaba %q", tc.stamped, tc.fromBuildInfo, got, tc.want)
			}
		})
	}
}

// Y el cableado: resolveVersion tiene que pasar por pickVersion. Lo único
// afirmable sin depender del entorno es que nunca deja escapar "(devel)" ni
// vacío.
func TestResolveVersion_NeverLeaksDevelOrEmpty(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })

	version = "dev"
	switch got := resolveVersion(); got {
	case "(devel)":
		t.Error("resolveVersion() devolvió \"(devel)\": ese valor no se le muestra a nadie")
	case "":
		t.Error("resolveVersion() devolvió vacío")
	}

	version = "v9.9.9"
	if got := resolveVersion(); got != "v9.9.9" {
		t.Errorf("resolveVersion() = %q con un valor estampado, se esperaba %q", got, "v9.9.9")
	}
}
