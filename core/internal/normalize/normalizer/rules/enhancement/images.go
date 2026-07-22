// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import (
	"strings"

	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
)

// ImagesRule normaliza rutas de imágenes placeholder
type ImagesRule struct {
	pathNormalizer *base.PathNormalizer
}

// NewImagesRule crea una nueva instancia de la regla
func NewImagesRule() *ImagesRule {
	return &ImagesRule{
		pathNormalizer: base.NewPathNormalizer(),
	}
}

func (r *ImagesRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")
	modified := false

	for i, line := range lines {
		// Buscar imágenes en la línea
		matches := r.findImageMatches(line)
		newLine := line

		for _, match := range matches {
			originalPath := match[2] // El path completo
			normalizedPath := r.pathNormalizer.NormalizeImagePath(originalPath)

			if originalPath != normalizedPath {
				// Reemplazar la ruta en la línea
				oldImageTag := match[0] // Todo el match ![alt](path)
				newImageTag := strings.Replace(oldImageTag, originalPath, normalizedPath, 1)
				newLine = strings.Replace(newLine, oldImageTag, newImageTag, 1)
				modified = true
			}
		}

		lines[i] = newLine
	}

	if modified {
		return strings.Join(lines, "\n"), nil
	}
	return content, nil
}

// findImageMatches encuentra todas las coincidencias de imágenes en una línea
func (r *ImagesRule) findImageMatches(line string) [][]string {
	var matches [][]string

	// Buscar patrones ![...](...) de forma básica
	start := 0
	for {
		imgStart := strings.Index(line[start:], "![")
		if imgStart == -1 {
			break
		}
		imgStart += start

		// Buscar el cierre del alt text
		altEnd := strings.Index(line[imgStart+2:], "](")
		if altEnd == -1 {
			break
		}
		altEnd += imgStart + 2

		// Buscar el cierre del path
		pathEnd := strings.Index(line[altEnd+2:], ")")
		if pathEnd == -1 {
			break
		}
		pathEnd += altEnd + 2

		// Extraer componentes
		fullMatch := line[imgStart : pathEnd+1]
		altText := line[imgStart+2 : altEnd]
		imagePath := line[altEnd+2 : pathEnd]

		// Verificar que sea una imagen válida
		if r.pathNormalizer.IsImagePath(imagePath) {
			matches = append(matches, []string{fullMatch, altText, imagePath, ""})
		}

		start = pathEnd + 1
	}

	return matches
}

func (r *ImagesRule) Description() string {
	return "Normaliza rutas de imágenes a la estructura assets/images/"
}

func (r *ImagesRule) Priority() int {
	return 4
}

func (r *ImagesRule) Category() base.RuleCategory {
	return base.CategoryEnhancement
}
