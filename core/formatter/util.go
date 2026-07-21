// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package formatter serializa un *ast.AST de vuelta a texto fuente
// canónico ("fmt --strict" / "fmt"): el inverso de parser.StrictParser
// (slidelang) y parser.DocumentFlexParser (doclang).
//
// Contrato (issue del MVP "fmt --strict — materializa el artefacto
// auditable"): la salida debe ser determinista (mismo AST → mismo texto
// byte a byte), idempotente (Format(Parse(Format(ast))) == Format(ast)) y
// semánticamente sin pérdida para los constructos que cada dialecto soporta
// hoy — no verbatim: el AST no retiene el whitespace/orden original, así
// que "canónico" significa reproducible, no preservación byte a byte del
// input.
package formatter

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// indent devuelve s con cada línea no vacía prefijada por n espacios.
// Las líneas vacías se dejan vacías (no espacios colgantes).
func indent(s string, n int) string {
	if s == "" {
		return s
	}
	prefix := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// quote envuelve s en comillas dobles SIN escapar nada — ni backslash ni
// comillas internas. Los lectores de valores entrecomillados en
// internal/elements/*.go (chart/map/directive/etc.) no implementan ningún
// mecanismo de escape: solo hacen strings.Trim(value, `"`) o buscan la
// PRÓXIMA comilla literal para cerrar el valor (parseCSVLine). Escapar `\`
// a `\\` (como hacía una versión anterior de este helper) corrompe el
// round-trip de cualquier valor que ya contenga un backslash literal
// (confirmado con examples/maps_special_characters_test.doclang: un
// details: "...\nfoo" con \n TIPEADO como texto, no como salto de línea,
// se duplicaba a "...\\\\nfoo" en cada pasada). Un valor que contenga `"`
// literal sigue sin poder round-trip-ear de forma perfecta — es una
// limitación pre-existente del dialecto, no algo que el formatter pueda
// resolver sin también tocar el parser para introducir un mecanismo de
// escape que hoy no existe.
func quote(s string) string {
	return `"` + s + `"`
}

// checkQuotable devuelve un error si s contiene `"` literal — la guarda
// que la limitación documentada en quote() implica: ese valor no puede
// round-trip-ear en ningún campo entrecomillado del dialecto strict, ya
// sea property de bloque (heading/title/subtitle/logo), IMAGE/TABLE,
// chart/map, o parámetro de directiva — los lectores correspondientes en
// internal/elements/*.go cierran el valor en la PRÓXIMA comilla literal
// que encuentran, así que el resto se trunca en silencio (o produce un
// error de sintaxis en un punto no relacionado del documento). Antes de
// que #206/#217 (transpiler flex→strict) existiera, esto era una
// limitación de bajo riesgo — un autor escribiendo texto strict a mano
// rara vez tipea comillas literales en estas posiciones. El transpiler
// expone estos mismos campos a texto flex/markdown libre, donde una
// comilla literal (p.ej. "cita" o contracciones) es común — así que la
// limitación pre-existente necesita una guarda explícita en cada función
// que va a llamar quote()/formatScalar()/formatInlineArray()/etc. sobre un
// campo user-controlled, en vez de quedar en silencio. Mismo principio que
// cada otro valor no representable en este dialecto (chart.Options, el
// contenido de QUOTE/CHECKLIST, GRID): reportar en vez de corromper.
func checkQuotable(nodeType, field, s string) error {
	if strings.Contains(s, `"`) {
		return newUnsupported(nodeType, fmt.Sprintf(
			"el campo %s contiene una comilla doble literal (%q) — el dialecto strict no tiene mecanismo de escape para campos entrecomillados (ver el comentario de quote() en formatter/util.go); el parser correspondiente truncaría o rompería el valor al reparsear en vez de preservarlo",
			field, s))
	}
	return nil
}

// sortedStringKeys ordena las claves de un map[string]interface{} — usado
// en todo punto donde se serializa un mapa Go, cuyo orden de iteración no
// es determinista, para garantizar salida byte-idéntica entre corridas.
func sortedStringKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// formatScalar serializa un valor escalar (string/bool/número) tal como lo
// esperan los parsers hand-rolled de chart/map/directive: strings
// entrecomillados, el resto tal cual con fmt.Sprint.
func formatScalar(v interface{}) string {
	switch val := v.(type) {
	case string:
		return quote(val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'g', -1, 64)
	case nil:
		return "null"
	default:
		return fmt.Sprint(val)
	}
}

// formatInlineArray serializa []string como ["a", "b", "c"] (forma inline
// usada por series:/labels: en charts, headers: de tablas, y otros arrays
// cortos). nodeType/field solo se usan para el mensaje de error si algún
// item no es quotable (ver checkQuotable).
func formatInlineArray(nodeType, field string, items []string) (string, error) {
	quoted := make([]string, len(items))
	for i, it := range items {
		if err := checkQuotable(nodeType, field, it); err != nil {
			return "", err
		}
		quoted[i] = quote(it)
	}
	return "[" + strings.Join(quoted, ", ") + "]", nil
}

// formatInlineRow serializa []interface{} como [a, b, "c"] — usado para
// filas de datos de chart (data: [...]), donde los elementos pueden ser
// number o string mezclados.
func formatInlineRow(nodeType, field string, row []interface{}) (string, error) {
	parts := make([]string, len(row))
	for i, v := range row {
		if s, ok := v.(string); ok {
			if err := checkQuotable(nodeType, field, s); err != nil {
				return "", err
			}
		}
		parts[i] = formatScalar(v)
	}
	return "[" + strings.Join(parts, ", ") + "]", nil
}

// formatFloat serializa un float64 sin ceros decimales innecesarios (40 en
// vez de 40.000000), coincidiendo con cómo strconv.ParseFloat en los
// parsers strict reconstruye el mismo valor sin importar cuántos decimales
// tenía el texto original (issue de determinismo: el AST no retiene la
// representación textual original, solo el float64).
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}
