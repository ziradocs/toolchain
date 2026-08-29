// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
	"go.ziradocs.com/core/v2/ast"
)

// formatFrontMatter serializa fm a un bloque "---\n...\n---\n" canónico,
// compartido entre slidelang (FormatStrict) y doclang (FormatDocument).
//
// Estrategia: reparsear fm.Raw (el YAML original tal cual lo vio
// parser.FrontMatterParser, ver parser/frontmatter.go:165) a un
// map[string]interface{} genérico y reserializarlo con claves ordenadas
// (yaml.Marshal ordena alfabéticamente los mapas), en vez de reconstruir el
// bloque entero campo por campo desde ast.FrontMatterNode. Reconstruirlo
// entero perdería en silencio cualquier clave de nivel superior que el AST no
// modela — y las hay de verdad: `lint_policy:` (ver
// linter.ResolvePolicyConfig, que la lee del propio fm.Raw) es la más
// importante.
//
// Sobre ese Raw se aplican DOS mapas, y la diferencia entre ellos es el
// corazón del issue #230:
//
//   - overrides PISA lo que traiga Raw. Es para las llaves cuyo campo tipado
//     es el valor COMPLETO de la llave: mode/title/author/date/theme/lang/
//     variables (escalares o mapa libre) y numbering/toc/page/watermark
//     (namespaces cerrados, donde el parser ya avisa FRONT005/FRONT006/
//     FRONT007 de que cualquier sub-llave desconocida se ignora). Así una
//     consumidora de la librería que mute FrontMatterNode.Title/.TOC/etc.
//     programáticamente (sin tocar Raw) ve su cambio reflejado.
//
//   - fallbacks solo RELLENA lo que falte en Raw. Es para header/footer/
//     layout_defaults, que son namespaces con fuga: rawFrontMatter los decodea
//     a structs tagueados, así que descarta toda sub-llave desconocida SIN
//     diagnóstico, y el corpus las trae de verdad (18.4_headers_footers_
//     advanced_flex.slidelang declara logo.link, text.content, divider,
//     page_numbers.prefix, social_links...). Pisarlas con el struct tipado
//     borraría la configuración del autor en un `fmt`, y el harness de
//     round-trip no lo vería: compara ASTs, y esas llaves nunca estuvieron en
//     el AST.
//
// El costo de esa segunda mitad, deliberado y no un olvido: mutar
// fm.HeaderFooter sobre un documento PARSEADO no se refleja en la salida. Es
// la misma forma de absorción silenciosa que el issue #230 arregla, aceptada
// acá porque la alternativa destruye datos del autor. Con Raw vacío —
// FrontMatterNode armado en código, o decodificado desde --format json, donde
// Raw es json:"-" — no falta ningún caso: toda llave está ausente, así que los
// cinco campos tipados se reconstruyen.
func formatFrontMatter(fm *ast.FrontMatterNode, overrides, fallbacks map[string]interface{}) (string, error) {
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
	for k, v := range fallbacks {
		if _, ok := data[k]; !ok {
			data[k] = v
		}
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

// asYAMLMap normaliza un struct de configuración a map[string]interface{}
// antes de meterlo en overrides/fallbacks. No es cosmético: yaml.v3 emite los
// structs en orden de DECLARACIÓN de campos y los mapas en orden ALFABÉTICO,
// así que meter el struct crudo rompe la idempotencia de `fmt` justo en la
// ruta que el issue #230 arregla. Con Raw vacío la primera pasada emitiría
// `header:` desde el struct (enabled, height, background, text, logo, border);
// la segunda, al reparsear esa salida, lo tomaría de Raw ya como mapa
// (background, border, enabled, height, logo, text). Mismo AST, dos textos.
//
// Vía yaml y no un round-trip por JSON a propósito: mantiene int como int (ni
// StartFrom ni TOCConfig.Depth pasan por float64), y son los tags `yaml:` de
// ast/nodes.go — espejo de los `json:` — los que hacen el trabajo real de
// emitir los nombres de llave que el parser acepta.
//
// Devuelve v tal cual si la conversión falla; un error acá solo puede venir de
// un tipo no serializable, que en esta familia de structs no existe, y el
// formatter no tiene por qué morir por una normalización cosmética.
func asYAMLMap(v interface{}) interface{} {
	out, err := yaml.Marshal(v)
	if err != nil {
		return v
	}
	var m map[string]interface{}
	if err := yaml.Unmarshal(out, &m); err != nil || m == nil {
		return v
	}
	return m
}

// frontMatterOverrides arma el mapa de campos bien tipados que deben GANAR
// sobre Raw — ver el doc comment de formatFrontMatter para el criterio de por
// qué una llave va acá y no en frontMatterFallbacks. Omite los campos en su
// valor cero para no forzar claves vacías que Raw no tenía.
//
// Todo campo exportado de ast.FrontMatterNode tiene que aparecer acá, en
// frontMatterFallbacks, o en la lista de exclusiones de
// TestFrontMatterOverridesCoverAllTypedFields. Ese guard existe porque este es
// el TERCER campo que se cuela: Lang (ver frontmatter_lang_test.go) y después
// Numbering/HeaderFooter/TOC/Page/Watermark (issue #230) desaparecían en
// silencio de la salida cuando Raw estaba vacío.
func frontMatterOverrides(fm *ast.FrontMatterNode, mode string) map[string]interface{} {
	overrides := map[string]interface{}{}
	// mode gana sobre fm.Mode cuando el llamador lo fuerza: FormatStrict y
	// FormatDocumentStrict pasan "strict" sin importar qué diga el AST,
	// porque esos dos formatters siempre emiten el dialecto strict. Pero
	// FormatDocument pasa "" — no fuerza ningún dialecto, flex/flex-full/auto
	// son sinónimos para documentos (ver CLAUDE.md) — y antes de este fix eso
	// significaba que fm.Mode NUNCA se consideraba por ese camino: un AST con
	// Mode="flex-full" y Raw vacío (construido en código, o decodificado
	// desde --format json) se formateaba sin `mode:`, y al reparsear el
	// resultado el dialecto se perdía (hallazgo de code review).
	//
	// El fallback excluye el valor "auto" a propósito — NO por Raw vacío,
	// que resultó ser el criterio equivocado (segunda vuelta de code
	// review): un frontmatter genuinamente vacío (`---\n---`, documento
	// real, jamás pasó por código) también deja Raw == "" y por lo tanto
	// también se confundía con un AST construido en código. "auto" es la
	// señal correcta porque es el ÚNICO valor que FrontMatterParser rellena
	// en silencio (`raw.Mode = "auto"`, FRONT001) cuando el documento NO
	// declara `mode:` — así que cualquier documento real sin `mode:`
	// explícito, tenga Raw vacío o con otras llaves, resuelve a exactamente
	// ese valor, nunca a otro. Excluirlo cierra los dos huecos a la vez: no
	// hornea `mode: auto` en un documento que nunca lo pidió (ni con
	// frontmatter vacío ni con frontmatter parcial), y sigue preservando
	// cualquier valor que sí distingue algo real (flex-full, strict, o un
	// "auto" que el propio Raw ya declaraba explícitamente, que de todos
	// modos sobrevive por el pass-through normal de Raw).
	if mode != "" {
		overrides["mode"] = mode
	} else if fm.Mode != "" && fm.Mode != "auto" {
		overrides["mode"] = fm.Mode
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
	if fm.Lang != "" {
		overrides["lang"] = fm.Lang
	}
	if len(fm.Variables) > 0 {
		overrides["variables"] = fm.Variables
	}
	// numbering se emite en su forma booleana, no en la forma-mapa legacy
	// (`numbering: {enabled: true, style: 1.1.1}`) que todavía emite
	// `doclang init --template technical`: el AST solo modela el bool, y
	// `style` no tiene ningún consumidor en el toolchain (ver rawNumbering en
	// parser/frontmatter.go). Formatear canoniza la forma legacy y pierde esa
	// clave inerte.
	if fm.Numbering != nil {
		overrides["numbering"] = *fm.Numbering
	}
	if fm.TOC != nil {
		overrides["toc"] = asYAMLMap(fm.TOC)
	}
	if fm.Page != nil {
		overrides["page"] = asYAMLMap(fm.Page)
	}
	if fm.Watermark != nil {
		overrides["watermark"] = asYAMLMap(fm.Watermark)
	}
	return overrides
}

// frontMatterFallbacks arma el mapa de campos que solo se emiten si Raw NO
// trae ya la llave — ver el doc comment de formatFrontMatter para por qué
// header/footer/layout_defaults no pueden pisar.
//
// Ojo con la forma: fm.HeaderFooter NO es una llave de frontmatter. El parser
// arma ese struct a partir de TRES llaves de nivel superior (`header:`,
// `footer:`, `layout_defaults:`, ver FrontMatterParser.convertHeaderFooterConfig),
// así que emitir `header_footer:` sería una clave que nadie lee.
func frontMatterFallbacks(fm *ast.FrontMatterNode) map[string]interface{} {
	fallbacks := map[string]interface{}{}
	if fm.HeaderFooter == nil {
		return fallbacks
	}
	if fm.HeaderFooter.Header != nil {
		fallbacks["header"] = asYAMLMap(fm.HeaderFooter.Header)
	}
	if fm.HeaderFooter.Footer != nil {
		fallbacks["footer"] = asYAMLMap(fm.HeaderFooter.Footer)
	}
	if len(fm.HeaderFooter.LayoutDefaults) > 0 {
		fallbacks["layout_defaults"] = asYAMLMap(fm.HeaderFooter.LayoutDefaults)
	}
	return fallbacks
}
