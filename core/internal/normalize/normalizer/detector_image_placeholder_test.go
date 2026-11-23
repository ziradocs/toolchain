// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package normalizer

import "testing"

func countImagePlaceholders(r DetectionResult) int {
	n := 0
	for _, p := range r.Patterns {
		if p.Type == "image_placeholder" {
			n++
		}
	}
	return n
}

const imageDetectorHeader = "---\ntitle: Deck\nmode: flex\n---\n\n# Uno\n\n"

// Una URL, una ruta relativa al protocolo o una ruta desde la raíz no cuenta
// como placeholder aunque diga "image", "example" o "sample": un deck con
// una foto de images.unsplash.com no enciende el normalizador por eso.
func TestDetector_ImagePlaceholderIgnoresExternalAndAbsolutePaths(t *testing.T) {
	for name, path := range map[string]string{
		"https":                 "https://example.com/a.png",
		"images en el host":     "https://images.unsplash.com/photo-1.jpg",
		"esquema en mayúsculas": "HTTPS://images.example.com/sample.png",
		"data URI":              "data:image/svg+xml;utf8,example.svg",
		"relativa al protocolo": "//images.example.com/x.png",
		"desde la raíz":         "/api/assets/image/sample.png",
	} {
		t.Run(name, func(t *testing.T) {
			r := NewDetector().Detect(imageDetectorHeader + "![a](" + path + ")\n")
			if n := countImagePlaceholders(r); n != 0 {
				t.Errorf("%s contó como placeholder: %+v", path, r.Patterns)
			}
			if r.Detected {
				t.Errorf("%s encendió el normalizador (score %v): %+v", path, r.Score, r.Patterns)
			}
		})
	}
}

// Una ruta relativa placeholder sigue contando, también detrás de una URL en
// la misma línea, y la línea suma un solo patrón.
func TestDetector_ImagePlaceholderKeepsRelativePaths(t *testing.T) {
	for name, content := range map[string]string{
		"sola":            "![a](ruta/imagen.png)\n",
		"detrás de URL":   "![u](https://images.unsplash.com/p.jpg) ![a](path/to/example.png)\n",
		"dos en la línea": "![a](image1.png) ![b](image2.png)\n",
	} {
		t.Run(name, func(t *testing.T) {
			r := NewDetector().Detect(imageDetectorHeader + content)
			if n := countImagePlaceholders(r); n != 1 {
				t.Errorf("got %d patrones image_placeholder, want 1: %+v", n, r.Patterns)
			}
			if !r.Detected {
				t.Errorf("no encendió el normalizador (score %v)", r.Score)
			}
		})
	}
}

// Como antes, una ruta relativa sin palabra de placeholder no cuenta, y de
// cada línea solo se mira la primera imagen con ruta relativa.
func TestDetector_ImagePlaceholderIgnoresPlainRelativePath(t *testing.T) {
	for name, content := range map[string]string{
		"sola":                      "![a](img/logo.png)\n",
		"placeholder en la segunda": "![a](img/logo.png) ![b](ruta/imagen.png)\n",
	} {
		t.Run(name, func(t *testing.T) {
			r := NewDetector().Detect(imageDetectorHeader + content)
			if n := countImagePlaceholders(r); n != 0 {
				t.Errorf("contó como placeholder: %+v", r.Patterns)
			}
		})
	}
}
