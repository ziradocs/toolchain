// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func fullDiagramTheme() DiagramThemeColors {
	return DiagramThemeColors{
		NodeBG:      "#nodebg",
		NodeFG:      "#nodefg",
		NodeLine:    "#nodeline",
		Edge:        "#edge",
		EdgeLabelBG: "#edgelabelbg",
		AccentBG:    "#accentbg",
		ClusterBG:   "#clusterbg",
		NoteBG:      "#notebg",
		FontFamily:  "Inter",
	}
}

// TestMermaidThemeVariables_MatchesBrowserMapping fija el mapeo TOKEN→CLAVE
// exacto que buildMermaidThemeVariables emite en el navegador
// (slidelang/.../modules/mermaid.js, PR #228). La tabla se duplica acá a
// propósito, escrita a mano: si alguien cambia una clave de un lado, este
// test falla y obliga a cambiar el otro.
//
// Una revisión de #250 encontró que portar a Go solo la MITAD de un criterio
// equivalente para charts dejó un hueco que el navegador ya no tenía; este
// test existe para que ese modo de falla no se repita en silencio.
func TestMermaidThemeVariables_MatchesBrowserMapping(t *testing.T) {
	vars := fullDiagramTheme().MermaidThemeVariables()

	want := map[string]string{
		"mainBkg":             "#nodebg",
		"primaryColor":        "#nodebg",
		"primaryTextColor":    "#nodefg",
		"textColor":           "#nodefg",
		"primaryBorderColor":  "#nodeline",
		"nodeBorder":          "#nodeline",
		"lineColor":           "#edge",
		"edgeLabelBackground": "#edgelabelbg",
		"secondaryColor":      "#accentbg",
		"clusterBkg":          "#clusterbg",
		"clusterBorder":       "#clusterbg",
		"noteBkgColor":        "#notebg",
		"fontFamily":          "Inter",
	}
	if len(vars) != len(want) {
		t.Errorf("el mapeo emitió %d claves, esperaba %d: %#v", len(vars), len(want), vars)
	}
	for key, expected := range want {
		if vars[key] != expected {
			t.Errorf("themeVariables[%q] = %q, want %q", key, vars[key], expected)
		}
	}
}

// TestMermaidThemeVariables_ZeroValueEmitsNothing es la garantía de "sin
// tema, byte por byte igual": nil deja que el caller omita themeVariables
// por completo.
func TestMermaidThemeVariables_ZeroValueEmitsNothing(t *testing.T) {
	if vars := (DiagramThemeColors{}).MermaidThemeVariables(); vars != nil {
		t.Errorf("el zero value debe emitir nil, got %#v", vars)
	}
	if extras := (DiagramThemeColors{}).MermaidExtras(); extras != nil {
		t.Errorf("el zero value no debe aportar extras, got %#v", extras)
	}
	if got := MermaidInitConfigJS(true, (DiagramThemeColors{}).MermaidExtras()...); got != MermaidInitConfigJS(true) {
		t.Errorf("sin tema el object literal debe ser idéntico al de siempre:\n got %s\nwant %s", got, MermaidInitConfigJS(true))
	}
}

// TestMermaidThemeVariables_PartialTokensAndFontDefault comprueba que cada
// token es independiente y que la fuente cae al mismo literal que usa el
// navegador cuando el tema no declara font-main.
func TestMermaidThemeVariables_PartialTokensAndFontDefault(t *testing.T) {
	vars := DiagramThemeColors{Edge: "#solo-edge"}.MermaidThemeVariables()

	if vars["lineColor"] != "#solo-edge" {
		t.Errorf("lineColor = %q, want #solo-edge", vars["lineColor"])
	}
	if vars["fontFamily"] != "arial" {
		t.Errorf("fontFamily = %q, want arial (el mismo default del navegador)", vars["fontFamily"])
	}
	for _, ausente := range []string{"mainBkg", "primaryColor", "noteBkgColor", "clusterBkg"} {
		if _, existe := vars[ausente]; existe {
			t.Errorf("un token no declarado no debe emitir %q: %#v", ausente, vars)
		}
	}
}

// TestMermaidInitConfigJS_ThemeCannotBreakTheSecurityPair fija que el tema
// entra como MermaidExtra (serializado con encoding/json) y por lo tanto no
// puede ni romper el object literal ni anular securityLevel/htmlLabels, que
// se emiten al final — la invariante estructural del issue #85.
func TestMermaidInitConfigJS_ThemeCannotBreakTheSecurityPair(t *testing.T) {
	hostil := DiagramThemeColors{NodeBG: `"} ); alert(1); //`, Edge: "</script>"}
	got := MermaidInitConfigJS(true, hostil.MermaidExtras()...)

	if !strings.HasSuffix(got, "securityLevel: 'strict', htmlLabels: false }") {
		t.Errorf("el par de seguridad debe seguir cerrando el literal: %s", got)
	}
	if !strings.Contains(got, "themeVariables:") {
		t.Errorf("el tema no llegó al literal: %s", got)
	}

	// La comilla del payload tiene que salir ESCAPADA (\"), que es lo que la
	// deja como texto dentro del string JSON en vez de cerrarlo. Buscar la
	// subcadena `); alert(1)` a secas no sirve: aparece legítimamente dentro
	// del string ya escapado, así que un test así falla sobre código correcto
	// — es el error que tenía la primera versión de esta aserción.
	if !strings.Contains(got, `\"} ); alert(1); //`) {
		t.Errorf("la comilla del payload no quedó escapada: %s", got)
	}
	// Y lo que de verdad importa para una página HTML: nada puede cerrar el
	// <script> que envuelve esta llamada. encoding/json escapa < y > como
	// </> justamente por esto.
	if strings.Contains(got, "</script>") {
		t.Errorf("el valor pudo cerrar el bloque <script>: %s", got)
	}
	// La forma esperada se calcula con el mismo encoder en vez de
	// hardcodear la secuencia de escape: así el test dice "sale exactamente
	// como encoding/json lo escaparía" y no depende de cómo se escriba
	// < en el fuente del test.
	escaped, err := json.Marshal("</script>")
	if err != nil {
		t.Fatalf("marshal de referencia: %v", err)
	}
	if !strings.Contains(got, strings.Trim(string(escaped), `"`)) {
		t.Errorf("esperaba el cierre de script en su forma escapada (%s): %s", escaped, got)
	}
}

// ---------------------------------------------------------------------------
// Fuentes auto-hospedadas
// ---------------------------------------------------------------------------

// dataURI arma un data: URI de fuente con base64 real, que es lo que
// DiagramFontFace.valid() exige (el regex por sí solo no prueba que decodifique).
func dataURI(mime string, payload string) string {
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString([]byte(payload))
}

func fontTheme(faces ...DiagramFontFace) DiagramThemeColors {
	return DiagramThemeColors{FontFamily: "Inter", Fonts: faces}
}

// TestFontFaceCSS_EmitsUsableRule fija la forma de la regla: familia
// entrecomillada, src tal cual, format() DERIVADO del MIME y font-display
// block (ver el comentario de FontFaceCSS para por qué block y no swap).
func TestFontFaceCSS_EmitsUsableRule(t *testing.T) {
	src := dataURI("font/woff2", "bytes de la fuente")
	css := fontTheme(DiagramFontFace{Family: "Inter", Weight: "700", Style: "italic", Src: src}).FontFaceCSS()

	for _, want := range []string{
		"@font-face {",
		`font-family: "Inter";`,
		"src: url(" + src + `) format("woff2");`,
		"font-weight: 700;",
		"font-style: italic;",
		"font-display: block;",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("la regla no contiene %q:\n%s", want, css)
		}
	}
}

// TestFontFaceCSS_FormatComesFromMIME comprueba las cuatro extensiones
// soportadas. El format() NO viaja como campo justamente para que no pueda
// contradecir al contenido: acá se fija esa derivación.
func TestFontFaceCSS_FormatComesFromMIME(t *testing.T) {
	for mime, want := range map[string]string{
		"font/woff2": "woff2",
		"font/woff":  "woff",
		"font/ttf":   "truetype",
		"font/otf":   "opentype",
	} {
		css := fontTheme(DiagramFontFace{Family: "F", Src: dataURI(mime, "x")}).FontFaceCSS()
		if !strings.Contains(css, `format("`+want+`")`) {
			t.Errorf("%s debería emitir format(%q):\n%s", mime, want, css)
		}
	}
}

// TestFontFaceCSS_RejectsUnusableSources: una cara que no puede cargar se
// descarta ENTERA. Emitirla a medias deja al navegador cayendo al fallback en
// silencio, que es el modo de falla que motor-temas-v2.md §2.3 quiere quitar
// —y acá además significaría que Mermaid mide con métricas equivocadas—.
func TestFontFaceCSS_RejectsUnusableSources(t *testing.T) {
	casos := map[string]DiagramFontFace{
		"sin familia":        {Family: "  ", Src: dataURI("font/woff2", "x")},
		"sin src":            {Family: "F"},
		"mime no soportado":  {Family: "F", Src: dataURI("font/eot", "x")},
		"no es data:":        {Family: "F", Src: "https://fonts.example/f.woff2"},
		"no es base64":       {Family: "F", Src: "data:font/woff2;base64,no-es-base64!"},
		"base64 mal padeado": {Family: "F", Src: "data:font/woff2;base64,AAAAA"},
		"file://":            {Family: "F", Src: "file:///etc/passwd"},
	}
	for nombre, face := range casos {
		if css := fontTheme(face).FontFaceCSS(); css != "" {
			t.Errorf("%s debería descartarse, emitió:\n%s", nombre, css)
		}
		if sh := fontTheme(face).FontLoadShorthands(); len(sh) != 0 {
			t.Errorf("%s no debería aportar shorthand, dio %#v", nombre, sh)
		}
	}
}

// TestFontFaceCSS_FamilyCannotEscapeTheStyleBlock es la invariante de
// seguridad de esta pieza. El destino es un <style>, que en HTML es texto
// crudo: la secuencia `</style` lo cierra INCLUSO dentro de un string CSS, y
// la CSP de la página temporal trae 'unsafe-inline' para script. Un nombre de
// familia viene del tema, así que tiene que salir sin ningún `<` ni `>`
// literal.
func TestFontFaceCSS_FamilyCannotEscapeTheStyleBlock(t *testing.T) {
	hostil := `x</style><script>alert(1)</script>`
	css := fontTheme(DiagramFontFace{Family: hostil, Src: dataURI("font/woff2", "x")}).FontFaceCSS()

	if css == "" {
		t.Fatal("la cara era válida salvo por el nombre; no debía descartarse")
	}
	if strings.Contains(css, "<") || strings.Contains(css, ">") {
		t.Errorf("el nombre de familia dejó pasar un < o > literal:\n%s", css)
	}
	if strings.Contains(strings.ToLower(css), "</style") {
		t.Errorf("el nombre pudo cerrar el bloque <style>:\n%s", css)
	}
	// Y el escape es el de CSS, no un borrado: el nombre sigue siendo el
	// mismo code point para el navegador.
	if !strings.Contains(css, `\3c `) || !strings.Contains(css, `\3e `) {
		t.Errorf("esperaba escapes hexadecimales CSS, no caracteres perdidos:\n%s", css)
	}
	// La comilla y la barra siguen escapándose como antes.
	quoted := fontTheme(DiagramFontFace{Family: `a"b\c`, Src: dataURI("font/woff2", "x")}).FontFaceCSS()
	if !strings.Contains(quoted, `"a\"b\\c"`) {
		t.Errorf("comilla/barra sin escapar:\n%s", quoted)
	}
}

// TestFontLoadShorthands_AreValidCSSFontShorthands: document.fonts.load()
// RECHAZA un shorthand inválido en vez de cargar, así que la forma importa —
// orden style/weight/size/family, y un rango de pesos omitido porque es
// sintaxis de descriptor y no de la propiedad font.
func TestFontLoadShorthands_AreValidCSSFontShorthands(t *testing.T) {
	src := dataURI("font/woff2", "x")
	got := DiagramThemeColors{Fonts: []DiagramFontFace{
		{Family: "Inter", Src: src},
		{Family: "Inter", Weight: "700", Style: "italic", Src: src},
		{Family: "Inter Variable", Weight: "400 700", Src: src},
		{Family: "Inter", Weight: "normal", Style: "normal", Src: src},
		{Family: "Inter", Weight: "no-es-un-peso", Src: src},
	}}.FontLoadShorthands()

	want := []string{
		`16px "Inter"`,
		`italic 700 16px "Inter"`,
		`16px "Inter Variable"`, // el rango se omite: load() emparejaría igual la familia
		`normal 16px "Inter"`,   // style normal se omite, weight normal es válido en el shorthand
		`16px "Inter"`,          // peso inválido: se omite el descriptor, no la cara
	}
	if len(got) != len(want) {
		t.Fatalf("esperaba %d shorthands, got %d: %#v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("shorthand[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestIsZero_FontsOnlyThemeIsNotZero: al entrar Fonts el struct dejó de ser
// comparable y IsZero pasó a ser campo por campo. La consecuencia que importa
// es esta — un tema que SOLO declara fuentes tiene que seguir emitiendo
// themeVariables, o su fontFamily nunca llegaría a Mermaid.
func TestIsZero_FontsOnlyThemeIsNotZero(t *testing.T) {
	soloFuentes := DiagramThemeColors{Fonts: []DiagramFontFace{{Family: "Inter", Src: dataURI("font/woff2", "x")}}}

	if soloFuentes.IsZero() {
		t.Error("un tema con fuentes no es el zero value")
	}
	if soloFuentes.MermaidThemeVariables() == nil {
		t.Error("un tema con fuentes debe emitir themeVariables")
	}
	if !(DiagramThemeColors{}).IsZero() {
		t.Error("el zero value sí es cero")
	}
	// Y campo por campo sigue cubriendo cada color por separado.
	for nombre, tema := range map[string]DiagramThemeColors{
		"NodeBG": {NodeBG: "#000"}, "NodeFG": {NodeFG: "#000"}, "NodeLine": {NodeLine: "#000"},
		"Edge": {Edge: "#000"}, "EdgeLabelBG": {EdgeLabelBG: "#000"}, "AccentBG": {AccentBG: "#000"},
		"ClusterBG": {ClusterBG: "#000"}, "NoteBG": {NoteBG: "#000"}, "FontFamily": {FontFamily: "Inter"},
	} {
		if tema.IsZero() {
			t.Errorf("un tema con solo %s no es cero", nombre)
		}
	}
}

// TestCacheFingerprint_DigestsFontBytes es la lección de #250 en su forma
// específica para fuentes: el CONTENIDO del archivo tiene que entrar a la
// clave —cambiar la fuente conservando el nombre de familia cambia métricas y
// por lo tanto el SVG— pero entra como digest, no como los megabytes de
// base64.
func TestCacheFingerprint_DigestsFontBytes(t *testing.T) {
	uno := fontTheme(DiagramFontFace{Family: "Inter", Src: dataURI("font/woff2", "version A")})
	otro := fontTheme(DiagramFontFace{Family: "Inter", Src: dataURI("font/woff2", "version B")})

	if uno.CacheFingerprint() == otro.CacheFingerprint() {
		t.Error("dos archivos distintos con la misma familia deben dar fingerprints distintos")
	}
	if uno.CacheFingerprint() != fontTheme(DiagramFontFace{Family: "Inter", Src: dataURI("font/woff2", "version A")}).CacheFingerprint() {
		t.Error("el mismo tema debe dar el mismo fingerprint")
	}
	// Los bytes NO viajan en el fingerprint: solo su digest.
	if strings.Contains(uno.CacheFingerprint(), base64.StdEncoding.EncodeToString([]byte("version A"))) {
		t.Errorf("el fingerprint no debe cargar el base64 crudo:\n%s", uno.CacheFingerprint())
	}
	// Y los colores siguen distinguiéndose, que es lo que ya cubría el hash
	// anterior.
	if (DiagramThemeColors{Edge: "#111"}).CacheFingerprint() == (DiagramThemeColors{Edge: "#222"}).CacheFingerprint() {
		t.Error("dos colores distintos deben dar fingerprints distintos")
	}
}

// ---------------------------------------------------------------------------
// sanitizeFontFamilyList — font-main real que Mermaid, verificado, descarta
// ---------------------------------------------------------------------------

// TestSanitizeFontFamilyList_LeavesSafeIdentsUnquoted es la mitad
// "no rompas lo que ya funcionaba" del arreglo: un nombre simple no gana
// comillas que no tenía, y las palabras clave genéricas (sans-serif) y el
// caso especial -apple-system se dejan intactos — AMBOS dejan de
// reconocerse si se entrecomillan (verificado contra Chromium: entrecomillado
// pasan a buscar una fuente instalada literalmente llamada así).
func TestSanitizeFontFamilyList_LeavesSafeIdentsUnquoted(t *testing.T) {
	for _, raw := range []string{
		"Inter",
		"arial",
		"Open Sans",
		"-apple-system",
		"sans-serif",
		"system-ui",
		"-apple-system, BlinkMacSystemFont, sans-serif",
		`"Segoe UI", Roboto, sans-serif`,
	} {
		got := sanitizeFontFamilyList(raw)
		want := raw // sin comas de más que reordenar espacios: mismo contenido
		// normalizamos solo el espaciado alrededor de comas para comparar,
		// ya que el join siempre usa ", ".
		wantNorm := strings.Join(func() []string {
			var out []string
			for _, t := range splitFontFamilyList(raw) {
				out = append(out, strings.TrimSpace(t))
			}
			return out
		}(), ", ")
		if got != wantNorm {
			t.Errorf("sanitizeFontFamilyList(%q) = %q, want %q (raw: %q)", raw, got, wantNorm, want)
		}
	}
}

// TestSanitizeFontFamilyList_QuotesWhatMermaidWouldDiscard es la mitad
// positiva: cada caso de acá es un valor real que, sin entrecomillar,
// Mermaid descarta en silencio. Verificado empíricamente contra Chromium
// (no asumido): un apóstrofe sin comillas corrompe el valor de la custom
// property --mermaid-font-family que Mermaid arma vía insertRule (se traga
// la `}` de cierre), y `element.style.setProperty("font-family", valor)`
// —la otra ruta interna de Mermaid— ignora en silencio un identificador que
// empieza con guion seguido de dígito o con un dígito.
func TestSanitizeFontFamilyList_QuotesWhatMermaidWouldDiscard(t *testing.T) {
	casos := map[string]string{
		"apóstrofe":               "O'Brien Sans",
		"guion seguido de dígito": "-1Font",
		"empieza con dígito":      "123Font",
		"comilla doble suelta":    `Weird"Name`,
	}
	for nombre, raw := range casos {
		got := sanitizeFontFamilyList(raw)
		if len(got) < 2 || got[0] != '"' || got[len(got)-1] != '"' {
			t.Errorf("%s (%q): esperaba quedar entrecomillado, dio %q", nombre, raw, got)
		}
		// Y el contenido original sigue ahí, solo protegido por las comillas
		// (cssEscapeString escapa \" y \\, pero un apóstrofe no necesita
		// escape dentro de un string delimitado por comillas dobles).
		if !strings.Contains(got, "Sans") && nombre == "apóstrofe" {
			t.Errorf("se perdió contenido: %q -> %q", raw, got)
		}
	}
}

// TestSanitizeFontFamilyList_RespectsAuthorQuoting: una entrada que el autor
// YA escribió entre comillas —el caso común documentado en ResolveFontMain,
// un font-main con fallback tipo `'Inter', sans-serif`— se deja tal cual,
// sin volver a escaparla ni tocar su contenido.
func TestSanitizeFontFamilyList_RespectsAuthorQuoting(t *testing.T) {
	got := sanitizeFontFamilyList(`"L'Oreal Display", sans-serif`)
	want := `"L'Oreal Display", sans-serif`
	if got != want {
		t.Errorf("sanitizeFontFamilyList = %q, want %q (no debía tocar una entrada ya entrecomillada)", got, want)
	}
}

// TestSanitizeFontFamilyList_CommaInsideQuotesIsNotASeparator cubre el
// splitter: una coma DENTRO de un string CSS no separa la lista.
func TestSanitizeFontFamilyList_CommaInsideQuotesIsNotASeparator(t *testing.T) {
	tokens := splitFontFamilyList(`"Weird, Font Name", sans-serif`)
	if len(tokens) != 2 {
		t.Fatalf("esperaba 2 entradas, dio %d: %#v", len(tokens), tokens)
	}
	if strings.TrimSpace(tokens[0]) != `"Weird, Font Name"` {
		t.Errorf("la primera entrada perdió la coma interna: %q", tokens[0])
	}
}

// TestSanitizeFontFamilyList_SkipsEmptyEntries: una lista con coma colgante
// (`"Inter,"` o `"Inter, ,sans-serif"`) no debe producir un token vacío —el
// join uniría con ", " y dejaría comas dobles.
func TestSanitizeFontFamilyList_SkipsEmptyEntries(t *testing.T) {
	got := sanitizeFontFamilyList("Inter, , sans-serif,")
	if got != "Inter, sans-serif" {
		t.Errorf("sanitizeFontFamilyList = %q, want %q", got, "Inter, sans-serif")
	}
}

// TestMermaidThemeVariables_FontFamilyGoesThroughSanitizer fija que
// MermaidThemeVariables usa sanitizeFontFamilyList y no el valor crudo — la
// consecuencia real, al nivel del mapa que de verdad llega a
// mermaid.initialize().
func TestMermaidThemeVariables_FontFamilyGoesThroughSanitizer(t *testing.T) {
	vars := DiagramThemeColors{FontFamily: "O'Brien Sans"}.MermaidThemeVariables()
	if vars["fontFamily"] != `"O'Brien Sans"` {
		t.Errorf(`themeVariables["fontFamily"] = %q, want %q`, vars["fontFamily"], `"O'Brien Sans"`)
	}
}
