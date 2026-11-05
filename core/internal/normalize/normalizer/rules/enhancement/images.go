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
		// La línea se arma en una sola pasada con las posiciones de cada
		// imagen. Antes cada imagen hacía un strings.Replace sobre la línea
		// entera, y "![a](b.png)" repetido era cuadrático.
		var b strings.Builder
		last := 0
		for _, m := range r.findImageMatches(line) {
			originalPath := line[m.pathStart:m.pathEnd]
			normalizedPath := r.pathNormalizer.NormalizeImagePath(originalPath)
			if originalPath == normalizedPath {
				continue
			}
			// Como antes, se cambia la primera aparición de la ruta dentro
			// de ![alt](ruta), que puede caer en el alt si lo repite.
			oldImageTag := line[m.start:m.end]
			b.WriteString(line[last:m.start])
			b.WriteString(strings.Replace(oldImageTag, originalPath, normalizedPath, 1))
			last = m.end
		}
		if last > 0 {
			b.WriteString(line[last:])
			lines[i] = b.String()
			modified = true
		}
	}

	if modified {
		return strings.Join(lines, "\n"), nil
	}
	return content, nil
}

// imageMatch es una imagen ![alt](ruta) de una línea: [start, end) el tag
// completo y [pathStart, pathEnd) la ruta.
type imageMatch struct {
	start, end         int
	pathStart, pathEnd int
}

// findImageMatches encuentra todas las coincidencias de imágenes en una línea
func (r *ImagesRule) findImageMatches(line string) []imageMatch {
	var matches []imageMatch

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

		// Verificar que sea una imagen válida
		if r.pathNormalizer.IsImagePath(line[altEnd+2 : pathEnd]) {
			matches = append(matches, imageMatch{start: imgStart, end: pathEnd + 1, pathStart: altEnd + 2, pathEnd: pathEnd})
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

// AppliesTo implementa base.DialectScopedRule: la estructura assets/images/
// es una convención de slidelang para contenido generado por IA (las rutas
// placeholder que inventa un modelo). En un documento, una ruta relativa de
// imagen es la que escribió el autor y se resuelve contra el archivo fuente,
// como en cualquier herramienta de markdown; reescribirla perdía la imagen.
// Pegaba de lleno en un .md de proyecto común: sin frontmatter y arrancando
// en "# ", el Detector ya puntúa 0.6 y enciende el set entero de reglas.
//
// El normalizador no recibe la ruta del archivo fuente, así que "no
// reescribir si la imagen existe" no es decidible acá; por eso se apaga por
// dialecto y no por existencia.
func (r *ImagesRule) AppliesTo(d base.Dialect) bool {
	return d != base.DialectDocuments
}
