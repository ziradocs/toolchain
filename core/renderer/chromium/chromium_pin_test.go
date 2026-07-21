// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package chromium

import (
	"encoding/hex"
	"testing"
)

// Issue #76: el pin de Chromium (chromiumVersion + chromiumSHA256) puede
// quedar desactualizado en silencio si alguien edita una de las dos
// constantes sin la otra, o rompe el formato de una entrada del mapa de
// hashes. La frescura real (¿sigue siendo la versión Stable vigente?) la
// vigila el workflow .github/workflows/chromium-pin-check.yml contra
// chrome-for-testing; este test solo verifica la forma estructural del pin
// committeado, sin red.
func TestChromiumPin_HasExpectedPlatforms(t *testing.T) {
	wantPlatforms := []string{"darwin/arm64", "darwin/amd64", "linux/amd64", "windows/amd64"}

	if len(chromiumSHA256) != len(wantPlatforms) {
		t.Fatalf("chromiumSHA256 tiene %d entradas, se esperaban %d: %v", len(chromiumSHA256), len(wantPlatforms), chromiumSHA256)
	}

	for _, platform := range wantPlatforms {
		hash, ok := chromiumSHA256[platform]
		if !ok {
			t.Errorf("chromiumSHA256 no tiene entrada para la plataforma soportada %q", platform)
			continue
		}

		raw, err := hex.DecodeString(hash)
		if err != nil {
			t.Errorf("chromiumSHA256[%q] = %q no es hex válido: %v", platform, hash, err)
			continue
		}
		if len(raw) != 32 {
			t.Errorf("chromiumSHA256[%q] tiene %d bytes decodificados, un SHA-256 debe tener 32", platform, len(raw))
		}
	}
}

func TestChromiumPin_VersionIsNonEmpty(t *testing.T) {
	if chromiumVersion == "" {
		t.Fatal("chromiumVersion está vacío")
	}
}
