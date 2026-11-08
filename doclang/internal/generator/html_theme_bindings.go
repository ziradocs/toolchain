// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import "go.ziradocs.com/core/v2/renderer"

// Variables de superficie del HTML/PDF para temas externos.
//
// El CSS del documento (core/renderer/document_html.go) lee los fondos y
// colores de tablas, paneles, teclas, citas, alertas, vínculos y el margen
// del PDF de variables propias (--doclang-table-bg, --doclang-panel-bg,
// --doclang-body-link-color, ...), con el literal de siempre como fallback.
// No lee --doclang-link-color, --doclang-quote-*, --doclang-alert-* ni
// --doclang-page-bg para esos elementos porque los temas embebidos los
// definen con valores que nunca se aplicaron ahí: leerlos cambiaría su
// render.
//
// Un tema externo trae las variables del contrato, no las de superficie.
// htmlThemeVariables deriva estas de aquellas, solo para temas externos, sin
// pisar ninguna que el tema defina por su cuenta. Las reglas:
//
//   - Un fondo se toma del tema solo junto con un color de texto del tema:
//     el específico (--doclang-quote-color, --doclang-alert-info-color) o, si
//     no hay, --doclang-text-color. Un fondo oscuro con el texto oscuro del
//     literal sería ilegible.
//   - La tabla toma el fondo de la hoja; paneles y teclas, el de la fila
//     alterna si el tema lo define (es el tono de superficie que el tema ya
//     emparejó con su texto), o el de la hoja.
//   - Un vínculo dentro de una cita o una alerta toma el color de vínculo
//     del tema solo si esa superficie también tomó su fondo del tema; si no,
//     su fondo sigue siendo el claro literal y el vínculo, el del navegador.
//   - Algunas variables del contrato tienen un fallback claro que se pinta
//     bajo el texto del tema: la fila alterna y el hover de la tabla, la banda
//     de encabezado/pie de page-view y el fondo de quiz y poll. Si el tema
//     no las define, toman el fondo de la hoja (o el de la fila alterna). Lo
//     mismo con el texto de la opción correcta de un quiz cuando el tema da
//     --doclang-alert-success-bg sin su color.
//   - El fondo de las gráficas no se deriva: la imagen de la gráfica se
//     dibuja sobre blanco.
//
// Los valores se copian tal cual y pasan por el mismo saneo que cualquier
// variable del tema (renderer.SanitizeCSSCustomProperty); una fuente que no
// lo pasa no se copia (la propia variable ya avisa al emitirse).
func htmlThemeVariables(vars map[string]string, external bool) map[string]string {
	if !external || len(vars) == 0 {
		return vars
	}

	out := make(map[string]string, len(vars)+48)
	for k, v := range vars {
		out[k] = v
	}

	get := func(name string) (string, bool) {
		v, ok := vars["--doclang-"+name]
		if !ok || v == "" {
			return "", false
		}
		if _, safe := renderer.SanitizeCSSCustomProperty("--doclang-"+name, v); !safe {
			return "", false
		}
		return v, true
	}
	set := func(name, value string) {
		if _, defined := vars["--doclang-"+name]; !defined {
			out["--doclang-"+name] = value
		}
	}
	// setPair fija un fondo y su texto juntos: el texto específico o, si
	// no hay, el del cuerpo. Sin ninguno de los dos no fija nada.
	setPair := func(bgName, fgName, bg string, fgSources ...string) {
		for _, src := range fgSources {
			if fg, ok := get(src); ok {
				set(bgName, bg)
				set(fgName, fg)
				return
			}
		}
	}

	link, hasLink := get("link-color")
	if hasLink {
		set("body-link-color", link)
	}
	// linkOn da el color de vínculo a una superficie solo si su fondo ya
	// sale del tema (definido por él o derivado arriba).
	linkOn := func(bgName, linkName string) {
		if _, themed := out["--doclang-"+bgName]; themed && hasLink {
			set(linkName, link)
		}
	}
	text, hasText := get("text-color")

	sheet, hasSheet := get("page-surface-bg")
	if !hasSheet {
		sheet, hasSheet = get("page-bg")
	}
	tint, hasTint := get("table-stripe-bg")
	if !hasTint {
		tint, hasTint = sheet, hasSheet
	}
	if hasSheet {
		set("table-bg", sheet)
		set("header-footer-bg", sheet)
	}
	if hasTint {
		set("table-stripe-bg", tint)
		set("table-hover-bg", tint)
		set("panel-bg", tint)
		set("kbd-bg", tint)
		if _, quote := get("quote-bg"); !quote {
			set("quiz-bg", tint)
		}
	}
	if _, bg := get("alert-success-bg"); bg && hasText {
		if _, fg := get("alert-success-color"); !fg {
			set("quiz-correct-color", text)
		}
	}
	if border, ok := get("table-border"); ok {
		set("panel-border", border)
		set("kbd-border", border)
	}

	if bg, ok := get("quote-bg"); ok {
		setPair("blockquote-bg", "blockquote-color", bg, "quote-color", "text-color")
		if light, ok := get("text-light"); ok {
			set("blockquote-footer-color", light)
		}
	}
	if border, ok := get("quote-border"); ok {
		set("blockquote-border", border)
	}
	linkOn("blockquote-bg", "blockquote-link-color")

	for _, kind := range []string{"info", "warning", "danger", "success", "tip"} {
		if bg, ok := get("alert-" + kind + "-bg"); ok {
			setPair("callout-"+kind+"-bg", "callout-"+kind+"-color", bg, "alert-"+kind+"-color", "text-color")
		}
		if border, ok := get("alert-" + kind + "-border"); ok {
			set("callout-"+kind+"-border", border)
		}
		linkOn("callout-"+kind+"-bg", "callout-"+kind+"-link-color")
	}

	if hasText {
		set("element-title-color", text)
	}
	if light, ok := get("text-light"); ok {
		set("caption-color", light)
	}

	if bg, ok := get("code-bg"); ok {
		setPair("code-tab-active-bg", "code-tab-active-color", bg, "code-color")
	}

	if page, ok := get("page-bg"); ok {
		set("print-margin-bg", page)
	}

	return out
}
