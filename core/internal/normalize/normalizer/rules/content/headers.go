// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package content

import (
	"strings"

	"go.ziradocs.com/core/v2/internal/normalize/normalizer/base"
)

// HeadersRule convierte headers ## dentro del contenido a texto plano.
//
// Su premisa es el modelo de slides: un solo "##" titula el slide y cualquier
// otro "##" de la misma región delimitada por "---" es contenido, así que se
// degrada a "**negrita**". En doclang esa premisa es falsa — "#" abre una
// sección y los "##" de adentro son sus subsecciones, todas legítimas — y la
// regla se comía el segundo y siguientes "##" de cada sección: el heading
// desaparecía de la jerarquía y del TOC en `doclang build`, sin diagnóstico.
// De ahí AppliesTo, más abajo.
type HeadersRule struct {
	analyzer *base.DocumentAnalyzer
}

// NewHeadersRule crea una nueva instancia de la regla
func NewHeadersRule() *HeadersRule {
	return &HeadersRule{
		analyzer: base.NewDocumentAnalyzer(),
	}
}

func (r *HeadersRule) Apply(content string) (string, error) {
	lines := strings.Split(content, "\n")
	modified := false

	inSlideContent := false
	slideHasTitle := false
	startLine := r.analyzer.SkipFrontmatter(lines)

	for i := startLine; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Detectar separadores de slides
		if line == "---" {
			inSlideContent = false
			slideHasTitle = false
			continue
		}

		// Detectar título del slide (## después de normalización)
		if strings.HasPrefix(line, "## ") && !inSlideContent {
			slideHasTitle = true
			inSlideContent = false // Aún no estamos en contenido
			continue
		}

		// Si estamos en contenido del slide y encontramos otro ##
		if inSlideContent && strings.HasPrefix(line, "## ") {
			// Convertir a texto plano
			lines[i] = "**" + strings.TrimSpace(line[3:]) + "**"
			modified = true
		}

		// Cualquier línea no vacía después del título significa que estamos en contenido
		if slideHasTitle && line != "" && !strings.HasPrefix(line, "## ") {
			inSlideContent = true
		}
	}

	if modified {
		return strings.Join(lines, "\n"), nil
	}
	return content, nil
}

func (r *HeadersRule) Description() string {
	return "Convierte headers ## dentro del contenido del slide a texto plano"
}

func (r *HeadersRule) Priority() int {
	return 3 // Después de TitleSubtitleRule
}

func (r *HeadersRule) Category() base.RuleCategory {
	return base.CategoryContent
}

// AppliesTo implementa base.DialectScopedRule: esta regla solo tiene sentido
// sobre slides. Ver el doc comment del tipo, y el de base.Dialect para por qué
// el dialecto llega desde quien llama en vez de adivinarse acá.
//
// Ojo con "arreglarla" para doclang en vez de apagarla: el bug no es que le
// falte un caso, es que su modelo de "un título por bloque" no describe un
// documento. Un documento no tiene nada que esta regla pueda normalizar.
func (r *HeadersRule) AppliesTo(d base.Dialect) bool {
	return d != base.DialectDocuments
}
