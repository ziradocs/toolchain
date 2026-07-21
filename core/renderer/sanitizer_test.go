// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package renderer

import (
	"regexp"
	"strings"
	"testing"
)

func TestEscapeHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic HTML entities",
			input:    `<script>alert("XSS")</script>`,
			expected: `&lt;script&gt;alert(&quot;XSS&quot;)&lt;/script&gt;`,
		},
		{
			name:     "Ampersand",
			input:    "Tom & Jerry",
			expected: "Tom &amp; Jerry",
		},
		{
			name:     "Single quotes",
			input:    "It's a test",
			expected: "It&#39;s a test",
		},
		{
			name:     "All special characters",
			input:    `<div class="test" data-value='123' onclick="alert('xss')">Tom & Jerry</div>`,
			expected: `&lt;div class=&quot;test&quot; data-value=&#39;123&#39; onclick=&quot;alert(&#39;xss&#39;)&quot;&gt;Tom &amp; Jerry&lt;/div&gt;`,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "No special characters",
			input:    "Hello World",
			expected: "Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EscapeHTML(tt.input)
			if result != tt.expected {
				t.Errorf("EscapeHTML() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestEscapeHTMLAttribute(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Remove newlines",
			input:    "Line 1\nLine 2",
			expected: "Line 1Line 2",
		},
		{
			name:     "Remove carriage returns",
			input:    "Line 1\rLine 2",
			expected: "Line 1Line 2",
		},
		{
			name:     "Replace tabs with spaces",
			input:    "Col1\tCol2",
			expected: "Col1 Col2",
		},
		{
			name:     "HTML entities and whitespace",
			input:    "<script>\nalert('xss')\n</script>",
			expected: "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EscapeHTMLAttribute(tt.input)
			if result != tt.expected {
				t.Errorf("EscapeHTMLAttribute() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSanitizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Safe HTTP URL",
			input:    "http://example.com",
			expected: "http://example.com",
		},
		{
			name:     "Safe HTTPS URL",
			input:    "https://example.com/path?query=value",
			expected: "https://example.com/path?query=value",
		},
		{
			name:     "Mailto link",
			input:    "mailto:user@example.com",
			expected: "mailto:user@example.com",
		},
		{
			name:     "Tel link",
			input:    "tel:+1234567890",
			expected: "tel:+1234567890",
		},
		{
			name:     "Relative URL",
			input:    "/path/to/page",
			expected: "/path/to/page",
		},
		{
			name:     "JavaScript protocol - should be blocked",
			input:    "javascript:alert('XSS')",
			expected: "",
		},
		{
			name:     "JavaScript protocol uppercase - should be blocked",
			input:    "JAVASCRIPT:alert('XSS')",
			expected: "",
		},
		{
			name:     "JavaScript protocol mixed case - should be blocked",
			input:    "JaVaScRiPt:alert('XSS')",
			expected: "",
		},
		{
			name:     "Data URI - should be blocked",
			input:    "data:text/html,<script>alert('XSS')</script>",
			expected: "",
		},
		{
			name:     "VBScript protocol - should be blocked",
			input:    "vbscript:msgbox('XSS')",
			expected: "",
		},
		{
			name:     "File protocol - should be blocked",
			input:    "file:///etc/passwd",
			expected: "",
		},
		{
			name:     "Invalid URL - should be blocked",
			input:    "ht!tp://example.com",
			expected: "",
		},
		{
			name:     "Empty URL",
			input:    "",
			expected: "",
		},
		{
			name:     "URL with HTML entities",
			input:    "https://example.com?q=<script>",
			expected: "https://example.com?q=&lt;script&gt;",
		},
		{
			name:     "FTP URL",
			input:    "ftp://files.example.com",
			expected: "ftp://files.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeURL(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeURL() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestProcessInlineMarkdownSecure(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Bold text with XSS attempt",
			input:    "**<script>alert('xss')</script>**",
			expected: "<strong>&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;</strong>",
		},
		{
			name:     "Italic text",
			input:    "*hello world*",
			expected: "<em>hello world</em>",
		},
		{
			name:     "Code with HTML",
			input:    "`<div>code</div>`",
			expected: "<code>&lt;div&gt;code&lt;/div&gt;</code>",
		},
		{
			name:     "Link with safe URL",
			input:    "[Click here](https://example.com)",
			expected: `<a href="https://example.com">Click here</a>`,
		},
		{
			name:     "Link with javascript: URL should be blocked",
			input:    "[Click here](javascript:void(0))",
			expected: "Click here)", // The ) from void(0) is left because markdown doesn't support () in URLs
		},
		{
			name:     "Link with javascript: URL without parens",
			input:    "[Click](javascript:void)",
			expected: "Click",
		},
		{
			name:     "Highlight text",
			input:    "==important==",
			expected: "<mark>important</mark>",
		},
		{
			name:     "Strikethrough text",
			input:    "~~deleted~~",
			expected: "<del>deleted</del>",
		},
		{
			name:     "Mixed formatting",
			input:    "**bold** and *italic* and `code`",
			expected: "<strong>bold</strong> and <em>italic</em> and <code>code</code>",
		},
		{
			name:     "List items",
			input:    "- Item 1\n- Item 2\n- Item 3",
			expected: "<ul><li>Item 1</li><li>Item 2</li><li>Item 3</li></ul>",
		},
		{
			name:     "List with XSS attempt",
			input:    "- <script>alert('xss')</script>",
			expected: "<ul><li>&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;</li></ul>",
		},
		{
			name:     "Plain text with HTML",
			input:    "<img src=x onerror=alert('xss')>",
			expected: "&lt;img src=x onerror=alert(&#39;xss&#39;)&gt;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessInlineMarkdownSecure(tt.input)
			if result != tt.expected {
				t.Errorf("ProcessInlineMarkdownSecure() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestProcessInlineMarkdownFormatsSecure_NoEmptyEm cubre issue #12e1: la
// negrita se procesa antes que la cursiva, así que un "**" residual (p.ej.
// una negrita sin cerrar) puede llegar al regex de cursiva; antes, una
// captura vacía producía un "<em></em>" que sobrevivía al HTML generado.
func TestProcessInlineMarkdownFormatsSecure_NoEmptyEm(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "bare double asterisk", input: "**"},
		{name: "double asterisk mid-sentence", input: "a ** b"},
		{name: "unmatched bold before an italic run", input: "***x*"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessInlineMarkdownFormatsSecure(EscapeHTML(tt.input))
			if strings.Contains(result, "<em></em>") {
				t.Errorf("ProcessInlineMarkdownFormatsSecure(%q) = %q, produced an empty <em></em>", tt.input, result)
			}
			if strings.Contains(result, "<strong></strong>") {
				t.Errorf("ProcessInlineMarkdownFormatsSecure(%q) = %q, produced an empty <strong></strong>", tt.input, result)
			}
		})
	}
}

// TestProcessInlineMarkdownFormatsSecure_BoldItalicDelimiterRun cubre un
// hallazgo de code-review (bot) sobre el fix de issue #101: el patrón
// ***texto*** original (sin anclaje de contexto) consumía solo 3 de una
// racha de 4+ asteriscos consecutivos (p.ej. "****texto****"), dejando un
// "*" suelto a cada lado que las pasadas de negrita/cursiva de más abajo
// re-envolvían alrededor del HTML ya emitido por la pasada ***, produciendo
// anidado inválido/cruzado (p.ej. "<em><strong><em>texto</em></strong></em>"
// o similar). El patrón corregido exige un carácter sin "*" (o inicio/fin
// de texto) a cada lado del delimitador, así que una racha de 4+ no
// matchea aquí y cae al mismo comportamiento pre-existente (ya en main
// antes de este fix) que negrita/cursiva ya tenían para ese caso límite —
// ni mejor ni peor, pero sin la regresión de anidado inválido.
func TestProcessInlineMarkdownFormatsSecure_BoldItalicDelimiterRun(t *testing.T) {
	// Caso bien formado: debe anidar correctamente.
	wellFormed := ProcessInlineMarkdownFormatsSecure(EscapeHTML("***bold italic***"))
	if wellFormed != "<strong><em>bold italic</em></strong>" {
		t.Errorf("well-formed ***text*** = %q, want correctly-nested <strong><em>...</em></strong>", wellFormed)
	}

	// Rachas de 4+ asteriscos no deben producir anidado inválido/cruzado -
	// ninguna apertura de tag duplicada consecutiva del tipo que el bug
	// producía.
	tests := []string{
		"****texto****",
		"a ****b**** c",
		"*****fivestars*****",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			result := ProcessInlineMarkdownFormatsSecure(EscapeHTML(input))
			for _, bad := range []string{"<em><em>", "<strong><strong>", "<em><strong><em>", "<strong><em><em>"} {
				if strings.Contains(result, bad) {
					t.Errorf("ProcessInlineMarkdownFormatsSecure(%q) = %q, contains invalid doubled/crossed tag %q", input, result, bad)
				}
			}
		})
	}
}

// TestProcessInlineMarkdownFormatsSecure_NestedItalicInBold cubre issue #173:
// **texto *anidado*** (negrita que termina con una cursiva pegada a su
// cierre, fusionando 1+2 asteriscos en un run de 3) producía anidado
// cruzado/inválido <strong>...<em>...</strong></em> — el "**" no-greedy
// consumía solo 2 de los 3 asteriscos de cierre, dejando un "*" suelto que
// la pasada de cursiva reclamaba cruzando el "</strong>" ya emitido. En un
// documento real (examples/docx_format_complete_test.doclang) un único
// <strong> sin cerrar así arrastraba el resto del documento entero como
// contenido "bajo <strong>" para el validador de HTML (68 errores
// element-permitted-content + 6 close-order en cascada).
func TestProcessInlineMarkdownFormatsSecure_NestedItalicInBold(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bug real de issue #173",
			input:    "También podemos combinar formatos: **texto en negrita con *cursiva anidada***.",
			expected: "También podemos combinar formatos: <strong>texto en negrita con <em>cursiva anidada</em></strong>.",
		},
		{
			name:     "negrita simple sin cursiva anidada, no debe cambiar",
			input:    "**bold text**",
			expected: "<strong>bold text</strong>",
		},
		{
			name:     "italic simple, no debe cambiar",
			input:    "*italic text*",
			expected: "<em>italic text</em>",
		},
		{
			name:     "***bold italic*** (issue #101) no debe verse afectado",
			input:    "***bold italic***",
			expected: "<strong><em>bold italic</em></strong>",
		},
		{
			name:     "mirror case (cursiva-fuera, negrita-dentro) ya funcionaba y no debe cambiar",
			input:    "*italic text with **bold***",
			expected: "<em>italic text with <strong>bold</strong></em>",
		},
		{
			name:     "solo la negrita con cursiva pegada al cierre se ve afectada, la primera negrita no",
			input:    "**one** and **two *nested***",
			expected: "<strong>one</strong> and <strong>two <em>nested</em></strong>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessInlineMarkdownFormatsSecure(EscapeHTML(tt.input))
			if result != tt.expected {
				t.Errorf("ProcessInlineMarkdownFormatsSecure(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}

	// Límite conocido y aceptado (mismo espíritu que el caso de 4+
	// asteriscos de #101): más de una cursiva anidada dentro de la misma
	// negrita no se arregla, pero tampoco debe empeorar ni crashear.
	multiNested := ProcessInlineMarkdownFormatsSecure(EscapeHTML("**a *b* c *d***"))
	if multiNested == "" {
		t.Error("multi-nested-italic input produced empty output")
	}

	// Rachas de 4+ asteriscos (el caso ya cubierto por
	// TestProcessInlineMarkdownFormatsSecure_BoldItalicDelimiterRun) no deben
	// regresar con este nuevo patrón — repetido aquí porque este patrón es
	// justamente el que casi regresiona ese test durante el desarrollo del
	// fix (matcheaba parcialmente a mitad de la racha).
	for _, input := range []string{"****texto****", "a ****b**** c", "*****fivestars*****"} {
		result := ProcessInlineMarkdownFormatsSecure(EscapeHTML(input))
		for _, bad := range []string{"<em><em>", "<strong><strong>", "<em><strong><em>", "<strong><em><em>"} {
			if strings.Contains(result, bad) {
				t.Errorf("ProcessInlineMarkdownFormatsSecure(%q) = %q, contains invalid doubled/crossed tag %q", input, result, bad)
			}
		}
	}
}

// TestProcessInlineMarkdownSecureMultiline cubre la regresión encontrada en
// code-review sobre issue #12d: al cambiar text/quote de ProcessInlineMarkdownSecure
// (block) a una variante inline-only para evitar HTML de bloque anidado en un
// <p>, una quote multilínea perdía sus <br> y las líneas quedaban unidas por
// un simple salto de línea crudo (que el navegador colapsa a un espacio).
// Esta variante preserva "\n" -> "<br>" sin nunca tratar "- " como viñeta ni
// emitir HTML de bloque.
func TestProcessInlineMarkdownSecureMultiline(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single line",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "multi-line joins with br",
			input:    "Line one\nLine two",
			expected: "Line one<br>Line two",
		},
		{
			name:     "does not treat a leading dash as a bullet",
			input:    "- item one\n- item two",
			expected: "- item one<br>- item two",
		},
		{
			name:     "applies inline formatting per line",
			input:    "**bold**\n*italic*",
			expected: "<strong>bold</strong><br><em>italic</em>",
		},
		{
			name:     "escapes HTML",
			input:    "<script>alert(1)</script>",
			expected: "&lt;script&gt;alert(1)&lt;/script&gt;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessInlineMarkdownSecureMultiline(tt.input)
			if result != tt.expected {
				t.Errorf("ProcessInlineMarkdownSecureMultiline(%q) = %q, want %q", tt.input, result, tt.expected)
			}
			if strings.Contains(result, "<ul>") || strings.Contains(result, "<li>") {
				t.Errorf("ProcessInlineMarkdownSecureMultiline(%q) must never emit block HTML, got %q", tt.input, result)
			}
		})
	}
}

func TestProcessVariablesSecure(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		variables map[string]interface{}
		expected  string
	}{
		{
			name:      "Simple variable replacement",
			input:     "Hello {{name}}",
			variables: map[string]interface{}{"name": "World"},
			expected:  "Hello World",
		},
		{
			name:      "Variable with XSS content",
			input:     "Hello {{name}}",
			variables: map[string]interface{}{"name": "<script>alert('xss')</script>"},
			expected:  "Hello &lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;",
		},
		{
			name:      "Multiple variables",
			input:     "{{greeting}} {{name}}",
			variables: map[string]interface{}{"greeting": "Hello", "name": "World"},
			expected:  "Hello World",
		},
		{
			name:      "Variable not found",
			input:     "Hello {{unknown}}",
			variables: map[string]interface{}{"name": "World"},
			expected:  "Hello {{unknown}}",
		},
		{
			name:      "No variables",
			input:     "Hello World",
			variables: map[string]interface{}{},
			expected:  "Hello World",
		},
		{
			name:      "HTML in static text",
			input:     "<script>alert('xss')</script> {{name}}",
			variables: map[string]interface{}{"name": "World"},
			expected:  "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt; World",
		},
		{
			name:      "Nil variables",
			input:     "Hello {{name}}",
			variables: nil,
			expected:  "Hello {{name}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessVariablesSecure(tt.input, tt.variables)
			if result != tt.expected {
				t.Errorf("ProcessVariablesSecure() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestProcessTextWithVariablesAndMarkdownSecure(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		variables map[string]interface{}
		expected  string
	}{
		{
			name:      "Variable with markdown formatting",
			input:     "**{{name}}** is great",
			variables: map[string]interface{}{"name": "SlideLang"},
			expected:  "<strong>SlideLang</strong> is great",
		},
		{
			name:      "Variable with XSS and markdown",
			input:     "**{{name}}**",
			variables: map[string]interface{}{"name": "<script>alert('xss')</script>"},
			expected:  "<strong>&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;</strong>",
		},
		{
			name:      "Link with variable in URL",
			input:     "[Click]({{url}})",
			variables: map[string]interface{}{"url": "https://example.com"},
			expected:  `<a href="https://example.com">Click</a>`,
		},
		{
			name:      "Link with dangerous variable in URL",
			input:     "[Click]({{url}})",
			variables: map[string]interface{}{"url": "javascript:void"},
			expected:  "Click",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessTextWithVariablesAndMarkdownSecure(tt.input, tt.variables)
			if result != tt.expected {
				t.Errorf("ProcessTextWithVariablesAndMarkdownSecure() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSanitizeColor(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "3-digit hex", input: "#f00", expected: "#f00"},
		{name: "6-digit hex", input: "#ff0000", expected: "#ff0000"},
		{name: "8-digit hex with alpha", input: "#ff0000cc", expected: "#ff0000cc"},
		{name: "known CSS color name", input: "red", expected: "red"},
		{name: "known CSS color name mixed case", input: "Red", expected: "red"},
		{name: "empty string stays empty", input: "", expected: ""},
		{
			name:     "style/JS breakout attempt falls back",
			input:    "red'});fetch('http://169.254.169.254/');({a:'",
			expected: "#2196F3",
		},
		{name: "HTML breakout attempt falls back", input: `"><img src=x onerror=alert(1)>`, expected: "#2196F3"},
		{name: "unknown color name falls back", input: "notacolor", expected: "#2196F3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeColor(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeColor(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSanitizeCSSCustomProperty(t *testing.T) {
	tests := []struct {
		name      string
		propName  string
		value     string
		expectOK  bool
		wantValue string
	}{
		{name: "valid name and simple color value", propName: "--primary-color", value: "#ff0000", expectOK: true, wantValue: "#ff0000"},
		{name: "valid name with underscore and digits", propName: "--font_size2", value: "14px", expectOK: true, wantValue: "14px"},
		{name: "name missing -- prefix rejected", propName: "primary-color", value: "red", expectOK: false},
		{name: "name with space rejected", propName: "--primary color", value: "red", expectOK: false},
		{name: "value with closing brace rejected", propName: "--evil", value: `red; } </style><script>alert(1)</script>`, expectOK: false},
		{name: "value with angle bracket rejected", propName: "--evil", value: `red"><img src=x onerror=alert(1)>`, expectOK: false},
		{name: "value with semicolon rejected", propName: "--evil", value: "red; background: url(evil)", expectOK: false},
		{name: "value with newline rejected", propName: "--evil", value: "red\n}</style>", expectOK: false},
		{name: "legitimate font stack value", propName: "--font-family", value: "'Helvetica Neue', Arial, sans-serif", expectOK: true, wantValue: "'Helvetica Neue', Arial, sans-serif"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotValue, ok := SanitizeCSSCustomProperty(tt.propName, tt.value)
			if ok != tt.expectOK {
				t.Fatalf("SanitizeCSSCustomProperty(%q, %q) ok = %v, want %v", tt.propName, tt.value, ok, tt.expectOK)
			}
			if ok && gotValue != tt.wantValue {
				t.Errorf("SanitizeCSSCustomProperty(%q, %q) = %q, want %q", tt.propName, tt.value, gotValue, tt.wantValue)
			}
		})
	}
}

func TestProcessVariablesEscapeValues(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		variables map[string]interface{}
		expected  string
	}{
		{
			name:      "surrounding HTML is preserved untouched",
			input:     `<strong>Config</strong> {{name}}`,
			variables: map[string]interface{}{"name": "value"},
			expected:  `<strong>Config</strong> value`,
		},
		{
			name:      "variable value with HTML is escaped",
			input:     `Config {{evil}}`,
			variables: map[string]interface{}{"evil": "<script>alert(1)</script>"},
			expected:  `Config &lt;script&gt;alert(1)&lt;/script&gt;`,
		},
		{
			name:      "unknown variable left as literal placeholder",
			input:     `Config {{missing}}`,
			variables: map[string]interface{}{},
			expected:  `Config {{missing}}`,
		},
		{
			name:      "nil variables map returns text unchanged",
			input:     `<em>Config</em> {{name}}`,
			variables: nil,
			expected:  `<em>Config</em> {{name}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessVariablesEscapeValues(tt.input, tt.variables)
			if result != tt.expected {
				t.Errorf("ProcessVariablesEscapeValues(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestProcessInlineMarkdownFormatsSecure_BracketedSpans cubre la sintaxis de
// span con clase estilo pandoc [contenido]{.token}: cada token de la allowlist
// fija (inlineSpanTokens) produce su tag hard-coded, el contenido interno sigue
// recibiendo formato inline (negrita/cursiva/código), y —lo crítico— un token
// fuera de la allowlist o manipulado NO puede producir markup arbitrario: se
// deja el texto literal (ya escapado), nunca una clase/atributo inyectado.
func TestProcessInlineMarkdownFormatsSecure_BracketedSpans(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// --- Un caso por cada token de la allowlist ---
		{
			name:     "danger",
			input:    "[peligro]{.danger}",
			expected: `<span class="slidelang-text-danger">peligro</span>`,
		},
		{
			name:     "info",
			input:    "[nota]{.info}",
			expected: `<span class="slidelang-text-info">nota</span>`,
		},
		{
			name:     "success",
			input:    "[ok]{.success}",
			expected: `<span class="slidelang-text-success">ok</span>`,
		},
		{
			name:     "warning",
			input:    "[cuidado]{.warning}",
			expected: `<span class="slidelang-text-warning">cuidado</span>`,
		},
		{
			name:     "accent",
			input:    "[destacado]{.accent}",
			expected: `<span class="slidelang-text-accent">destacado</span>`,
		},
		{
			name:     "highlight-warning usa <mark>",
			input:    "[atención]{.highlight-warning}",
			expected: `<mark class="slidelang-highlight-warning">atención</mark>`,
		},
		{
			name:     "highlight-info usa <mark>",
			input:    "[dato]{.highlight-info}",
			expected: `<mark class="slidelang-highlight-info">dato</mark>`,
		},
		{
			name:     "highlight-success usa <mark>",
			input:    "[listo]{.highlight-success}",
			expected: `<mark class="slidelang-highlight-success">listo</mark>`,
		},
		{
			name:     "underline usa <u> sin clase",
			input:    "[subrayado]{.underline}",
			expected: `<u>subrayado</u>`,
		},
		{
			name:     "small usa <small>",
			input:    "[chico]{.small}",
			expected: `<small class="slidelang-text-small">chico</small>`,
		},
		{
			name:     "large",
			input:    "[grande]{.large}",
			expected: `<span class="slidelang-text-large">grande</span>`,
		},
		// --- Anidamiento: el contenido interno mantiene su formato ---
		{
			name:     "negrita dentro de span (span envuelve el formato)",
			input:    "[**importante**]{.danger}",
			expected: `<span class="slidelang-text-danger"><strong>importante</strong></span>`,
		},
		{
			name:     "negrita + texto plano dentro de span",
			input:    "[**bold** text]{.danger}",
			expected: `<span class="slidelang-text-danger"><strong>bold</strong> text</span>`,
		},
		{
			name:     "cursiva y código dentro de span",
			input:    "[usa *foo* y `bar`]{.info}",
			expected: `<span class="slidelang-text-info">usa <em>foo</em> y <code>bar</code></span>`,
		},
		{
			name:     "span dentro de negrita (formato envuelve el span)",
			input:    "**[peligro]{.danger}**",
			expected: `<strong><span class="slidelang-text-danger">peligro</span></strong>`,
		},
		{
			name:     "span en medio de texto normal",
			input:    "antes [medio]{.success} después",
			expected: `antes <span class="slidelang-text-success">medio</span> después`,
		},
		{
			name:     "dos spans en la misma línea",
			input:    "[a]{.danger} y [b]{.info}",
			expected: `<span class="slidelang-text-danger">a</span> y <span class="slidelang-text-info">b</span>`,
		},
		// --- Interacción span/enlace (P2 de PR #260: no debe cruzar tags) ---
		{
			name:     "span dentro del texto de un enlace: enlace bien formado con span adentro",
			input:    "[See [important]{.danger}](https://example.com)",
			expected: `<a href="https://example.com">See <span class="slidelang-text-danger">important</span></a>`,
		},
		{
			// El span no matchea (su contenido no puede cruzar el "[" interno),
			// así que el span degrada a inerte. La pasada de enlace, con su
			// propio patrón [^\]]+, matchea desde el "[" EXTERIOR y deja un "["
			// literal en el texto y "]{.danger}" literal detrás — un quirk
			// pre-existente del regex de enlaces con corchetes anidados, NO
			// introducido por los spans. Lo importante: el <a>…</a> queda
			// balanceado, sin ningún <span> cruzándolo (ver aserción explícita
			// de no-cruce más abajo).
			name:     "enlace dentro del contenido de un span: <a> balanceado, sin span cruzado",
			input:    "[See [here](https://example.com)]{.danger}",
			expected: `<a href="https://example.com">See [here</a>]{.danger}`,
		},
		{
			name:     "span y enlace adyacentes: ambos bien formados",
			input:    "[a]{.danger} [b](https://example.com)",
			expected: `<span class="slidelang-text-danger">a</span> <a href="https://example.com">b</a>`,
		},
		{
			name:     "corchete literal dentro del contenido no matchea (inerte, no cruzado)",
			input:    "[array[0]]{.info}",
			expected: "[array[0]]{.info}",
		},
		// --- Token desconocido: inerte, sin inyección ---
		{
			name:     "token fuera de la allowlist queda literal (no inyecta clase)",
			input:    "[texto]{.unknown}",
			expected: "[texto]{.unknown}",
		},
		{
			name:     "token que parece prefijo válido pero no está en el mapa",
			input:    "[texto]{.text-danger}",
			expected: "[texto]{.text-danger}",
		},
		{
			name:     "contenido escapado en token desconocido sigue escapado",
			input:    "[<b>x</b>]{.nope}",
			expected: "[&lt;b&gt;x&lt;/b&gt;]{.nope}",
		},
		// --- Intentos de inyección: no pueden producir markup arbitrario ---
		{
			name:     "token con espacio y handler no matchea (queda literal escapado)",
			input:    `[hola]{.danger x onmouseover=alert(1)}`,
			expected: `[hola]{.danger x onmouseover=alert(1)}`,
		},
		{
			name:     "token con comilla no matchea (comilla escapada, sin span)",
			input:    `[hola]{.danger"onmouseover="alert(1)}`,
			expected: `[hola]{.danger&quot;onmouseover=&quot;alert(1)}`,
		},
		{
			name:     "token con cierre de tag no matchea",
			input:    `[hola]{.danger><script>alert(1)</script>}`,
			expected: `[hola]{.danger&gt;&lt;script&gt;alert(1)&lt;/script&gt;}`,
		},
		{
			name:     "token vacío no matchea",
			input:    "[hola]{.}",
			expected: "[hola]{.}",
		},
		{
			name:     "contenido con XSS dentro de token válido sigue escapado",
			input:    `[<img src=x onerror=alert(1)>]{.danger}`,
			expected: `<span class="slidelang-text-danger">&lt;img src=x onerror=alert(1)&gt;</span>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessInlineMarkdownFormatsSecure(EscapeHTML(tt.input))
			if result != tt.expected {
				t.Errorf("ProcessInlineMarkdownFormatsSecure(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}

	// Ninguna combinación span/enlace debe producir HTML CRUZADO (P2 de PR
	// #260): p.ej. <span>See <a>important</span></a>, donde el </span> cierra
	// mientras un <a> abierto DENTRO del span sigue abierto. Un substring
	// ingenuo no basta (el anidado VÁLIDO <a><span>…</span></a> también
	// termina en "</span></a>"), así que validamos anidamiento LIFO real con
	// una pila de tags sobre la salida.
	crossInputs := []string{
		"[See [important]{.danger}](https://example.com)",
		"[See [here](https://example.com)]{.danger}",
		"[a]{.danger} [b](https://example.com)",
		"[**x** [y](https://example.com)]{.info}",
	}
	tagRe := regexp.MustCompile(`<(/?)([a-z0-9]+)[^>]*>`)
	for _, in := range crossInputs {
		out := ProcessInlineMarkdownFormatsSecure(EscapeHTML(in))
		var stack []string
		balanced := true
		for _, m := range tagRe.FindAllStringSubmatch(out, -1) {
			closing, name := m[1] == "/", m[2]
			if name == "br" { // void, sin cierre
				continue
			}
			if !closing {
				stack = append(stack, name)
				continue
			}
			if len(stack) == 0 || stack[len(stack)-1] != name {
				balanced = false
				break
			}
			stack = stack[:len(stack)-1]
		}
		if !balanced || len(stack) != 0 {
			t.Errorf("span/enlace %q produjo HTML con anidamiento inválido/cruzado: %q", in, out)
		}
	}

	// Emparejamiento de tags: el mapa inlineSpanTokens está escrito a mano, así
	// que un tag de cierre mal tecleado pasaría un check ingenuo de "contiene
	// <small". Verificamos que cada token abra y cierre el MISMO tag.
	pairs := []struct {
		token, open, close string
	}{
		{"danger", "<span", "</span>"},
		{"info", "<span", "</span>"},
		{"success", "<span", "</span>"},
		{"warning", "<span", "</span>"},
		{"accent", "<span", "</span>"},
		{"highlight-warning", "<mark", "</mark>"},
		{"highlight-info", "<mark", "</mark>"},
		{"highlight-success", "<mark", "</mark>"},
		{"underline", "<u>", "</u>"},
		{"small", "<small", "</small>"},
		{"large", "<span", "</span>"},
	}
	for _, p := range pairs {
		input := "[X]{." + p.token + "}"
		result := ProcessInlineMarkdownFormatsSecure(EscapeHTML(input))
		if !strings.HasPrefix(result, p.open) || !strings.HasSuffix(result, p.close) {
			t.Errorf("token %q: result %q no abre con %q y cierra con %q", p.token, result, p.open, p.close)
		}
		// Ningún token debe dejar un corchete/llave literal en la salida válida.
		if strings.ContainsAny(result, "[]{}") {
			t.Errorf("token %q: result %q contiene delimitadores de span sin consumir", p.token, result)
		}
	}
}
