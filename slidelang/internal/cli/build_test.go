// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFormats(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []string
	}{
		{"single format", "json", []string{"json"}},
		{"two formats", "html,json", []string{"html", "json"}},
		{"whitespace around commas", "html, json", []string{"html", "json"}},
		{"duplicate formats deduplicated", "json,html,json", []string{"json", "html"}},
		{"trailing comma ignored", "html,", []string{"html"}},
		{"empty string", "", nil},
		{"single comma only", ",", nil},
		{"all commas", ",,,", nil},
		{"three distinct formats", "html,json,pdf", []string{"html", "json", "pdf"}},
		{"case-insensitive", "HTML,Json", []string{"html", "json"}},
		{"case-insensitive duplicate deduplicated", "html,HTML", []string{"html"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFormats(tt.raw)
			if len(got) != len(tt.want) {
				t.Fatalf("parseFormats(%q) = %v, want %v", tt.raw, got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("parseFormats(%q)[%d] = %q, want %q", tt.raw, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestRunBuild_MultiFormat cubre issue #10: --format html,json debe generar ambos
// archivos en una sola invocación de build (un solo parseo del AST).
func TestRunBuild_MultiFormat(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "test.slidelang")
	source := `---
title: Test Multi Format
mode: flex
---

# Slide 1

Contenido de prueba.
`
	if err := os.WriteFile(inputFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test input file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "dist")
	opts := &BuildOptions{
		InputFile: inputFile,
		OutputDir: outputDir,
		Format:    "html,json",
		Mode:      "auto",
		LogLevel:  "error",
		NoColors:  true,
	}

	if err := runBuild(opts, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("runBuild() error = %v", err)
	}

	htmlPath := filepath.Join(outputDir, "test.html")
	jsonPath := filepath.Join(outputDir, "test.json")

	if _, err := os.Stat(htmlPath); err != nil {
		t.Errorf("expected HTML output at %s: %v", htmlPath, err)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("expected JSON output at %s: %v", jsonPath, err)
	}
}

// TestRunBuild_InvalidFormat_ReturnsError asegura que un formato inválido dentro
// de una lista falla con un error claro en vez de generarse silenciosamente.
func TestRunBuild_InvalidFormat_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "test.slidelang")
	source := `---
title: Test Invalid Format
mode: flex
---

# Slide 1

Contenido.
`
	if err := os.WriteFile(inputFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test input file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "dist")
	opts := &BuildOptions{
		InputFile: inputFile,
		OutputDir: outputDir,
		Format:    "html,bogus",
		Mode:      "auto",
		LogLevel:  "error",
		NoColors:  true,
	}

	if err := runBuild(opts, nil, nil, nil, nil, nil); err == nil {
		t.Fatal("expected error for invalid format in list, got nil")
	}

	// Regression guard: an invalid format anywhere in the list must be
	// rejected before generating ANY of the earlier, valid formats - an
	// invalid entry must not leave a partial (but real-looking) build on
	// disk while the command still reports failure.
	if _, err := os.Stat(filepath.Join(outputDir, "test.html")); !os.IsNotExist(err) {
		t.Errorf("expected no output written when the format list contains an invalid entry, but test.html exists (err=%v)", err)
	}
}

// TestRunBuild_BlankFormatList_ReturnsError cubre el path "no output format
// specified": un --format que solo contiene comas (p. ej. ",") parsea a una
// lista vacía vía parseFormats, y debe fallar con un error claro en vez de
// generar silenciosamente algún formato por defecto.
func TestRunBuild_BlankFormatList_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "test.slidelang")
	source := `---
title: Test Blank Format
mode: flex
---

# Slide 1

Contenido.
`
	if err := os.WriteFile(inputFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test input file: %v", err)
	}

	opts := &BuildOptions{
		InputFile: inputFile,
		OutputDir: filepath.Join(tmpDir, "dist"),
		Format:    ",",
		Mode:      "auto",
		LogLevel:  "error",
		NoColors:  true,
	}

	err := runBuild(opts, nil, nil, nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for a blank comma-only format list, got nil")
	}
	if err.Error() != "no output format specified" {
		t.Errorf("error = %q, want %q", err.Error(), "no output format specified")
	}
}

// TestRunBuild_RecognizedButUnimplementedFormat_CombinedWithValid_NoPartialOutput
// cubre un caso encontrado al revisar esta PR: "slidelang" (pretty-print) es
// un formato reconocido (pasa la validación de nombres válidos) pero no
// implementado en generator.GenerateWithOptions. Combinarlo con un formato
// que sí funciona (p. ej. "html") reproducía el mismo bug de build parcial
// en disco que la validación de nombres desconocidos ya arregla - "html" se
// generaba antes de fallar en el no implementado. Debe rechazarse por
// completo antes de generar nada. Antes usaba "pdf" como el ejemplo de
// formato no implementado; issue #128 lo implementó, así que este test pasó
// a "slidelang" (el único que queda sin implementar) para seguir cubriendo
// la misma garantía.
func TestRunBuild_RecognizedButUnimplementedFormat_CombinedWithValid_NoPartialOutput(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "test.slidelang")
	source := `---
title: Test Unimplemented Combo
mode: flex
---

# Slide 1

Contenido.
`
	if err := os.WriteFile(inputFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test input file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "dist")
	opts := &BuildOptions{
		InputFile: inputFile,
		OutputDir: outputDir,
		Format:    "html,slidelang",
		Mode:      "auto",
		LogLevel:  "error",
		NoColors:  true,
	}

	if err := runBuild(opts, nil, nil, nil, nil, nil); err == nil {
		t.Fatal("expected error when combining 'html' with the not-yet-implemented 'slidelang', got nil")
	}

	if _, err := os.Stat(filepath.Join(outputDir, "test.html")); !os.IsNotExist(err) {
		t.Errorf("expected no output written when the format list combines a valid format with an unimplemented one, but test.html exists (err=%v)", err)
	}
}

// TestRunBuild_HTMLContentRendersCorrectly cubre issue #71: builds.go's
// generador migró a html/template (#67) para auto-escaping contextual, pero
// el único test que ejercita ese pipeline real (TestRunBuild_MultiFormat)
// solo verifica que el archivo exista (os.Stat), nunca su contenido — una
// regresión de doble-escape (p.ej. un helper que debería retornar
// template.HTML pero retorna string plano) pasaría go build/go test
// limpiamente, ya que Execute() solo falla en errores estructurales, no en
// una salida válida-pero-incorrecta. Este test construye un fixture real
// (bold/italic/***, un título con caracteres especiales, un chart con JSON,
// y una imagen con alt) y asevera sobre el HTML string resultante.
func TestRunBuild_HTMLContentRendersCorrectly(t *testing.T) {
	tmpDir := t.TempDir()
	inputFile := filepath.Join(tmpDir, "test.slidelang")
	source := `---
title: "Reporte <Q1> & Resultados \"2026\""
mode: flex
---

# Formato de Texto

Este texto tiene **negrita**, *cursiva* y ***negrita cursiva*** combinadas.

![Diagrama <principal> & "detalles"](assets/diagram.png)

<<chart: bar>>
  data: [
    ["Q1", 10, 20]
  ]
  series: ["A", "B"]
`
	if err := os.WriteFile(inputFile, []byte(source), 0644); err != nil {
		t.Fatalf("failed to write test input file: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "dist")
	opts := &BuildOptions{
		InputFile: inputFile,
		OutputDir: outputDir,
		Format:    "html",
		Mode:      "auto",
		LogLevel:  "error",
		NoColors:  true,
		EnableAI:  true,
	}

	if err := runBuild(opts, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("runBuild() error = %v", err)
	}

	htmlBytes, err := os.ReadFile(filepath.Join(outputDir, "test.html"))
	if err != nil {
		t.Fatalf("failed to read generated HTML: %v", err)
	}
	html := string(htmlBytes)

	// Bold+italic combinado (issue #101): debe anidar correctamente, no
	// cruzado (<strong><em>x</strong></em>) ni con asteriscos residuales.
	if !strings.Contains(html, "<strong><em>negrita cursiva</em></strong>") {
		t.Errorf("expected correctly-nested <strong><em>negrita cursiva</em></strong> in output, not found in:\n%s", html)
	}
	if strings.Contains(html, "<em>negrita cursiva</strong></em>") {
		t.Error("found the #101 mis-nested <strong>/<em> pattern in output")
	}
	// Negrita y cursiva simples deben seguir funcionando sin interferencia
	// del nuevo patrón ***.
	if !strings.Contains(html, "<strong>negrita</strong>") {
		t.Error("expected simple <strong>negrita</strong> in output")
	}
	if !strings.Contains(html, "<em>cursiva</em>") {
		t.Error("expected simple <em>cursiva</em> in output")
	}

	// El título con &/</" debe aparecer escapado exactamente una vez, nunca
	// sin escapar (XSS/HTML-breakout) ni doble-escapado (&amp;lt; en vez de
	// &lt;).
	if strings.Contains(html, "<Q1>") {
		t.Error("title's raw <Q1> leaked into output unescaped")
	}
	if strings.Contains(html, "&amp;lt;") || strings.Contains(html, "&amp;amp;") {
		t.Error("title appears double-escaped in output")
	}
	if !strings.Contains(html, "&lt;Q1&gt;") {
		t.Errorf("expected escaped title &lt;Q1&gt; in output, not found in:\n%s", html)
	}

	// El <script type="application/json" id="slidelang-metadata"> debe seguir
	// siendo JSON válido pese a interpolar el título con caracteres
	// especiales.
	metaStart := strings.Index(html, `id="slidelang-metadata">`)
	if metaStart == -1 {
		t.Fatal("slidelang-metadata script tag not found in output")
	}
	metaStart += len(`id="slidelang-metadata">`)
	metaEnd := strings.Index(html[metaStart:], "</script>")
	if metaEnd == -1 {
		t.Fatal("closing </script> for slidelang-metadata not found")
	}
	metaJSON := html[metaStart : metaStart+metaEnd]
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(metaJSON), &parsed); err != nil {
		t.Errorf("slidelang-metadata script content is not valid JSON: %v\ncontent:\n%s", err, metaJSON)
	}
}
