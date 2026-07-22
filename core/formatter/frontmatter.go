// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/ast"
)

// formatFrontMatter serializa fm a un bloque "---\n...\n---\n" canónico,
// compartido entre slidelang (FormatStrict) y doclang (FormatDocument).
//
// Estrategia: reparsear fm.Raw (el YAML original tal cual lo vio
// parser.FrontMatterParser, ver parser/frontmatter.go:165) a un
// map[string]interface{} genérico y reserializarlo con claves ordenadas
// (gopkg.in/yaml.v3.Marshal ordena alfabéticamente los mapas — verificado
// antes de asumirlo, ver yaml_helpers.go), en vez de reconstruir cada
// sub-estructura (header/footer/layout_defaults, y cualquier clave
// custom no modelada en ast.FrontMatterNode como "toc"/"numbering" en
// frontmatter de doclang) a mano campo por campo. La reconstrucción a
// mano es exactamente el bug que este approach evita: FrontMatterNode solo
// tipa mode/title/author/date/theme/variables/header/footer — cualquier
// otra clave YAML de nivel superior (p. ej. "toc:"/"numbering:" en un
// frontmatter de doclang real, ver examples/docx_format_technical.doclang)
// se pierde en silencio si el formatter reconstruye desde los campos
// tipados en vez de desde el texto original.
//
// overrides gana sobre lo que traiga Raw para los campos bien tipados —
// así una consumidora de la librería que mute FrontMatterNode.Title/etc.
// programáticamente (sin tocar Raw) sigue viendo su cambio reflejado; todo
// lo demás (custom keys, header/footer) pasa a través de Raw sin cambios.
func formatFrontMatter(fm *ast.FrontMatterNode, overrides map[string]interface{}) (string, error) {
	if fm == nil {
		return "", nil
	}

	data := map[string]interface{}{}
	if strings.TrimSpace(fm.Raw) != "" {
		if err := yaml.Unmarshal([]byte(fm.Raw), &data); err != nil {
			return "", fmt.Errorf("formatter: no se pudo reparsear el frontmatter original: %w", err)
		}
		if data == nil {
			data = map[string]interface{}{}
		}
	}

	for k, v := range overrides {
		data[k] = v
	}

	body, err := yaml.Marshal(data)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	b.WriteString("---\n")
	b.Write(body)
	b.WriteString("---\n")
	return b.String(), nil
}

// frontMatterOverrides arma el mapa de campos bien tipados que deben ganar
// sobre Raw — omite los que están en su valor cero para no forzar claves
// vacías que Raw no tenía.
func frontMatterOverrides(fm *ast.FrontMatterNode, mode string) map[string]interface{} {
	overrides := map[string]interface{}{}
	if mode != "" {
		overrides["mode"] = mode
	}
	if fm.Title != "" {
		overrides["title"] = fm.Title
	}
	if fm.Author != "" {
		overrides["author"] = fm.Author
	}
	if fm.Date != "" {
		overrides["date"] = fm.Date
	}
	if fm.Theme != "" {
		overrides["theme"] = fm.Theme
	}
	if len(fm.Variables) > 0 {
		overrides["variables"] = fm.Variables
	}
	return overrides
}
