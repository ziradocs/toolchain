// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// DiagramThemeColors agrupa los tokens diagram-* de motor-temas-v2.md §2.2
// para el camino offline/PDF. Zero value = sin tema: reproduce el render
// anterior byte por byte, igual que ChartThemeColors.
//
// Existe porque el rasterizador offline de Mermaid nunca ve el CSS del deck:
// corre Mermaid dentro de Chromium sobre una página temporal propia, y un SVG
// generado por Mermaid no resuelve custom properties. El único canal es
// themeVariables en mermaid.initialize(), que es justo lo que
// MermaidThemeVariables construye.
type DiagramThemeColors struct {
	NodeBG      string // diagram-node-bg
	NodeFG      string // diagram-node-fg
	NodeLine    string // diagram-node-line
	Edge        string // diagram-edge
	EdgeLabelBG string // diagram-edge-label-bg
	AccentBG    string // diagram-accent-bg
	ClusterBG   string // diagram-cluster-bg
	NoteBG      string // diagram-note-bg
	// FontFamily NO es un token diagram-*: viene de font-main, y se incluye
	// acá porque el mapeo del navegador también lo emite (metadata.themeFontMain
	// en buildMermaidThemeVariables). Sin él, el mismo tema daría una fuente
	// en el deck y otra en el PNG/SVG offline.
	FontFamily string
	// Fonts son las fuentes auto-hospedadas que el tema declara, embebidas
	// como data: URI — ver DiagramFontFace para por qué el nombre de familia
	// por sí solo no alcanza.
	Fonts []DiagramFontFace
}

// IsZero reporta si no hay ningún color ni fuente de tema puesto.
//
// Campo por campo y no `d == DiagramThemeColors{}`: al entrar Fonts, el
// struct dejó de ser comparable. Que sea explícito tiene además una
// consecuencia deseada — un tema que SOLO declara fuentes (ningún color) no
// es cero, así que sigue emitiendo themeVariables con su fontFamily en vez de
// caer al camino "sin tema".
func (d DiagramThemeColors) IsZero() bool {
	return d.NodeBG == "" && d.NodeFG == "" && d.NodeLine == "" &&
		d.Edge == "" && d.EdgeLabelBG == "" && d.AccentBG == "" &&
		d.ClusterBG == "" && d.NoteBG == "" && d.FontFamily == "" &&
		len(d.Fonts) == 0
}

// mermaidDefaultFontFamily es el literal que el módulo del navegador emite
// cuando el tema no declara font-main. Se replica para que las dos rutas
// arranquen del mismo default en vez de que offline caiga al de Mermaid.
const mermaidDefaultFontFamily = "arial"

// MermaidThemeVariables traduce los tokens al vocabulario de themeVariables
// de Mermaid. Es un PORT LITERAL de buildMermaidThemeVariables en
// slidelang/internal/generator/template/assets/js/modules/mermaid.js (PR
// #228), que a su vez implementa la tabla "Mapeo a Mermaid" de
// motor-temas-v2.md §2.2 — mismas claves, mismos tokens, mismo default de
// fontFamily.
//
// Que sea un port literal y no una reinterpretación es deliberado: una
// revisión de #250 encontró que portar a Go solo la MITAD del criterio
// equivalente para charts dejó un hueco que el navegador ya no tenía. Si esta
// tabla cambia, tiene que cambiar en los dos lados a la vez.
//
// Devuelve nil con el zero value, para que el caller pueda omitir
// themeVariables por completo y el object literal quede idéntico al de
// siempre.
func (d DiagramThemeColors) MermaidThemeVariables() map[string]string {
	if d.IsZero() {
		return nil
	}
	vars := map[string]string{"fontFamily": mermaidDefaultFontFamily}
	if d.FontFamily != "" {
		vars["fontFamily"] = d.FontFamily
	}
	set := func(value string, keys ...string) {
		if value == "" {
			return
		}
		for _, key := range keys {
			vars[key] = value
		}
	}
	set(d.NodeBG, "mainBkg", "primaryColor")
	set(d.NodeFG, "primaryTextColor", "textColor")
	set(d.NodeLine, "primaryBorderColor", "nodeBorder")
	set(d.Edge, "lineColor")
	set(d.EdgeLabelBG, "edgeLabelBackground")
	set(d.AccentBG, "secondaryColor")
	set(d.ClusterBG, "clusterBkg", "clusterBorder")
	set(d.NoteBG, "noteBkgColor")
	return vars
}

// MermaidExtras devuelve los MermaidExtra que este tema aporta a
// MermaidInitConfigJS, o nil si no hay tema. Se pasa como extra —y no
// concatenado— para que el valor se serialice con encoding/json y ningún
// color pueda romper el object literal, y para que el par de seguridad
// (securityLevel/htmlLabels) siga emitiéndose al final y siga siendo
// inanulable (issue #85).
func (d DiagramThemeColors) MermaidExtras() []MermaidExtra {
	vars := d.MermaidThemeVariables()
	if vars == nil {
		return nil
	}
	return []MermaidExtra{{Key: "themeVariables", Value: vars}}
}

// ---------------------------------------------------------------------------
// Fuentes auto-hospedadas
// ---------------------------------------------------------------------------

// DiagramFontFace es una fuente auto-hospedada del tema, ya embebida como
// data: URI, lista para emitirse como @font-face dentro de la página temporal
// que rasteriza el diagrama.
//
// Existe porque FontFamily SOLO transporta un nombre, y un nombre no basta:
// la página temporal es un documento aparte (about:blank +
// Page.setDocumentContent, AL-5) que no hereda ni el CSS del deck ni sus
// @font-face. Sin el recurso, Mermaid mide texto con la fuente fallback y
// hornea esas métricas —anchos de nodo, wrapping, trazado de aristas— dentro
// del propio SVG/PNG. Un SVG de offline-assets se carga como <img>, que es
// otro documento aún dentro del deck: tampoco hereda nada. Y aunque
// offline-inline sí resuelva glifos con la fuente del padre, el layout ya se
// calculó con las métricas equivocadas.
//
// Src tiene que ser un data: URI: la página corre bajo una CSP sin red y sin
// file://, y "auto-hospedar siempre" (motor-temas-v2.md §2.3) ya es la regla
// del lado del deck.
type DiagramFontFace struct {
	Family string // font-family
	Weight string // opcional: "normal", "bold", 1-1000 o un rango "lo hi"
	Style  string // opcional: "normal", "italic", "oblique"
	Src    string // data:font/woff2;base64,... (ver dataFontURIRe)
}

// fontMIMEToFormat mapea el MIME de un data: URI a la palabra clave de
// format(). El formato se DERIVA del MIME en vez de viajar como campo aparte
// para que no puedan desincronizarse: un format() que no corresponde al
// contenido falla a cargar en algunos navegadores sin ningún error visible.
var fontMIMEToFormat = map[string]string{
	"font/woff2": "woff2",
	"font/woff":  "woff",
	"font/ttf":   "truetype",
	"font/otf":   "opentype",
}

// dataFontURIRe acota Src a un data: URI de fuente con base64 bien formado.
// Es allowlist estricta a propósito: este valor se escribe crudo dentro de un
// url(...) en un <style>, así que la garantía tiene que venir de la forma
// aceptada y no de intentar neutralizar lo rechazado.
var dataFontURIRe = regexp.MustCompile(`^data:(font/(?:woff2|woff|ttf|otf));base64,([A-Za-z0-9+/]+={0,2})$`)

// fontWeightNumberRe acepta un número de font-weight, 1-1000 (rango de CSS
// Fonts Level 4, no solo los múltiplos tradicionales de 100 — hay fuentes
// variables con pesos más finos).
var fontWeightNumberRe = regexp.MustCompile(`^([1-9][0-9]{0,2}|1000)$`)

// validatedFontWeight refleja el validador homónimo de slidelang
// (internal/generator/css/themes/fonts.go): "normal", "bold", un número
// 1-1000, o un rango "lo hi" de descriptor de fuente variable. Cualquier otra
// cosa se omite — mejor sin descriptor (cae al default de 400) que con uno
// que el navegador rechaza.
func validatedFontWeight(w string) string {
	w = strings.TrimSpace(w)
	if w == "" || w == "normal" || w == "bold" {
		return w
	}
	parts := strings.Fields(w)
	switch len(parts) {
	case 1:
		if fontWeightNumberRe.MatchString(parts[0]) {
			return parts[0]
		}
	case 2:
		lo, loOK := parts[0], fontWeightNumberRe.MatchString(parts[0])
		hi, hiOK := parts[1], fontWeightNumberRe.MatchString(parts[1])
		if loOK && hiOK {
			l, _ := strconv.Atoi(lo)
			h, _ := strconv.Atoi(hi)
			if l <= h {
				return lo + " " + hi
			}
		}
	}
	return ""
}

func validatedFontStyle(s string) string {
	switch s = strings.TrimSpace(s); s {
	case "", "normal", "italic", "oblique":
		return s
	}
	return ""
}

// cssEscapeString serializa s como string CSS entre comillas dobles.
//
// Escapa `<` y `>` como escapes hexadecimales CSS además de la comilla y la
// barra: el destino es un <style>, que en HTML es un elemento de texto crudo
// donde la secuencia `</style` cierra el bloque INCLUSO dentro de un string
// CSS. Sin esto, un tema con una familia llamada `x</style><script>...`
// escaparía a un contexto de script (y la CSP de estas páginas trae
// 'unsafe-inline'). El espacio final del escape es parte de la sintaxis: CSS
// consume hasta 6 dígitos hex, así que `\3c` seguido de una letra hex se
// leería como otro code point.
func cssEscapeString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r == '<':
			b.WriteString(`\3c `)
		case r == '>':
			b.WriteString(`\3e `)
		case r < 0x20 || r == 0x7f:
			// Un control char no aporta nada a un nombre de familia y sí
			// complica razonar sobre el contexto: se descarta.
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// valid reporta si la cara puede emitirse, y devuelve sus piezas ya
// validadas. Una cara inválida se descarta entera en vez de emitirse a
// medias: un @font-face con un src que no carga deja al navegador cayendo al
// fallback en silencio, que es justo el modo de falla que §2.3 quiere quitar.
func (f DiagramFontFace) valid() (format string, ok bool) {
	if strings.TrimSpace(f.Family) == "" {
		return "", false
	}
	m := dataFontURIRe.FindStringSubmatch(f.Src)
	if m == nil {
		return "", false
	}
	// El base64 tiene que decodificar de verdad: un padding mal puesto pasa
	// el regex pero produce una fuente corrupta que el navegador descarta
	// —otra vez— sin error visible.
	if _, err := base64.StdEncoding.DecodeString(m[2]); err != nil {
		return "", false
	}
	format, ok = fontMIMEToFormat[m[1]]
	return format, ok
}

// FontFaceCSS emite una regla @font-face por cada fuente válida, para
// inyectarla en el <style> de la página temporal. Devuelve "" si no hay
// ninguna.
//
// font-display es fijo en "block" y no viaja como campo: en un rasterizador
// no hay ningún caso en que convenga pintar con la fuente fallback y cambiar
// después — ese "después" ya no existe cuando la captura salió. Es la única
// diferencia deliberada con el @font-face que slidelang emite para el deck,
// donde "swap" sí es lo correcto porque hay un lector esperando.
func (d DiagramThemeColors) FontFaceCSS() string {
	var b strings.Builder
	for _, face := range d.Fonts {
		format, ok := face.valid()
		if !ok {
			continue
		}
		b.WriteString("@font-face {\n")
		b.WriteString("  font-family: " + cssEscapeString(face.Family) + ";\n")
		fmt.Fprintf(&b, "  src: url(%s) format(%q);\n", face.Src, format)
		if w := validatedFontWeight(face.Weight); w != "" {
			b.WriteString("  font-weight: " + w + ";\n")
		}
		if s := validatedFontStyle(face.Style); s != "" {
			b.WriteString("  font-style: " + s + ";\n")
		}
		b.WriteString("  font-display: block;\n")
		b.WriteString("}\n")
	}
	return b.String()
}

// FontLoadShorthands devuelve un shorthand de font por cada fuente válida,
// del tipo que espera document.fonts.load().
//
// Hace falta una llamada explícita porque un @font-face NO se descarga hasta
// que algo lo usa, y en esta página nada lo usa hasta que Mermaid dibuja —
// que es exactamente lo que se quiere evitar. document.fonts.ready por sí
// solo resuelve de inmediato sobre una página sin cargas pendientes.
//
// Un rango de pesos ("400 700") se omite del shorthand: es sintaxis válida
// para el descriptor de @font-face pero no para la propiedad font, y un
// shorthand inválido hace que load() rechace en vez de cargar. Sin el peso,
// load() empareja igual la familia.
func (d DiagramThemeColors) FontLoadShorthands() []string {
	var out []string
	for _, face := range d.Fonts {
		if _, ok := face.valid(); !ok {
			continue
		}
		var parts []string
		if s := validatedFontStyle(face.Style); s != "" && s != "normal" {
			parts = append(parts, s)
		}
		if w := validatedFontWeight(face.Weight); w != "" && len(strings.Fields(w)) == 1 {
			parts = append(parts, w)
		}
		parts = append(parts, "16px", cssEscapeString(face.Family))
		out = append(out, strings.Join(parts, " "))
	}
	return out
}

// CacheFingerprint devuelve una forma estable y compacta del tema, para que
// quien derive una clave de cache no tenga que conocer esta estructura.
//
// Los bytes de cada fuente se sustituyen por su digest: el contenido SÍ tiene
// que entrar a la clave —cambiar el archivo conservando el nombre de familia
// cambia métricas, y por lo tanto el SVG— pero hashear megabytes de base64
// por cada diagrama no aporta nada sobre hashear su digest.
func (d DiagramThemeColors) CacheFingerprint() string {
	type face struct{ Family, Weight, Style, SrcDigest string }
	fp := struct {
		DiagramThemeColors
		Fonts []face
	}{DiagramThemeColors: d}
	fp.DiagramThemeColors.Fonts = nil
	for _, f := range d.Fonts {
		sum := sha256.Sum256([]byte(f.Src))
		fp.Fonts = append(fp.Fonts, face{f.Family, f.Weight, f.Style, hex.EncodeToString(sum[:])})
	}
	// json.Marshal de un map ordena las claves, y acá son structs: la salida
	// es determinista entre corridas y entre máquinas.
	out, _ := json.Marshal(fp)
	return string(out)
}
