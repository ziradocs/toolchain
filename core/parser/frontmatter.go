// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"go.ziradocs.com/core/v2/a11y"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

const FrontMatterDelimiter = "---"

type FrontMatterParser struct {
	diagnostics []diagnostics.Diagnostic
}

type rawFrontMatter struct {
	Mode      string                 `yaml:"mode"`
	Title     string                 `yaml:"title"`
	Author    string                 `yaml:"author"`
	Date      string                 `yaml:"date"`
	Theme     string                 `yaml:"theme"`
	Lang      string                 `yaml:"lang"`
	Numbering *rawNumbering          `yaml:"numbering"`
	Variables map[string]interface{} `yaml:"variables"`
	// Configuración de headers y footers
	Header         *rawHeaderConfig            `yaml:"header"`
	Footer         *rawFooterConfig            `yaml:"footer"`
	LayoutDefaults map[string]*rawLayoutConfig `yaml:"layout_defaults"`
}

// rawNumbering accepts either shape a document's `numbering:` key has ever
// used: the current tri-state bool (`numbering: true`/`numbering: false`)
// or the legacy map form that predates it (`numbering:\n  enabled: true\n
// style: 1.1.1`, still emitted by `doclang init`'s `technical`/`report`
// templates as of #100's review). Both resolve to the same *bool.
//
// `style` in the map form has no consumer anywhere in this codebase
// (confirmed via grep across core/ and doclang/) — it was always
// aspirational. It's accepted and silently ignored rather than rejected, so
// existing front matter that declares it doesn't start failing builds.
type rawNumbering struct {
	// Enabled is nil when the map form is present but omits `enabled`
	// (`numbering: {}`) — that shape carries no information, so it's
	// treated the same as `numbering:` being absent entirely, not as an
	// implicit `false`.
	Enabled *bool
}

func (n *rawNumbering) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var b bool
		if err := value.Decode(&b); err != nil {
			return fmt.Errorf("numbering: expected true/false or a map with 'enabled', got %q", value.Value)
		}
		n.Enabled = &b
		return nil
	case yaml.MappingNode:
		var legacy struct {
			Enabled *bool       `yaml:"enabled"`
			Style   interface{} `yaml:"style"` // aspirational, no consumer; accepted and ignored
		}
		if err := value.Decode(&legacy); err != nil {
			return fmt.Errorf("numbering: %w", err)
		}
		n.Enabled = legacy.Enabled
		return nil
	default:
		return fmt.Errorf("numbering: expected a bool (true/false) or a map with 'enabled', got %v", value.Tag)
	}
}

// rawHeaderConfig mapea la configuración YAML de headers
type rawHeaderConfig struct {
	Enabled    bool                 `yaml:"enabled"`
	Height     string               `yaml:"height"`
	Background string               `yaml:"background"`
	Text       *rawHeaderFooterText `yaml:"text"`
	Logo       *rawLogoConfig       `yaml:"logo"`
	Border     *rawBorderConfig     `yaml:"border"`
}

// rawFooterConfig mapea la configuración YAML de footers
type rawFooterConfig struct {
	Enabled     bool                  `yaml:"enabled"`
	Height      string                `yaml:"height"`
	Background  string                `yaml:"background"`
	Text        *rawHeaderFooterText  `yaml:"text"`
	PageNumbers *rawPageNumbersConfig `yaml:"page_numbers"`
	Border      *rawBorderConfig      `yaml:"border"`
}

// rawHeaderFooterText mapea el contenido de texto de un header/footer. Acepta
// también un escalar (`text: "Some title"`) como atajo de `center` —
// `doclang init`'s `report` template emitió justo esa forma (issue #115) y no
// hay razón para rechazar en el parser lo que un `init` viejo ya generó.
// Mismo patrón que rawNumbering (más abajo en este archivo, precedente de
// #100): switch sobre value.Kind, pointer receiver.
type rawHeaderFooterText struct {
	Left   string `yaml:"left"`
	Center string `yaml:"center"`
	Right  string `yaml:"right"`
}

// rawHeaderFooterTextFields es un tipo definido (no un alias) sobre
// rawHeaderFooterText: decodificar en él en la rama MappingNode reutiliza los
// mismos tags yaml sin duplicar la lista de campos, y al ser un tipo nuevo no
// hereda UnmarshalYAML, así que no hay recursión infinita.
type rawHeaderFooterTextFields rawHeaderFooterText

func (t *rawHeaderFooterText) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var s string
		if err := value.Decode(&s); err != nil {
			return fmt.Errorf("text: expected a string or a map with left/center/right, got %q", value.Value)
		}
		t.Center = s
		return nil
	case yaml.MappingNode:
		var fields rawHeaderFooterTextFields
		if err := value.Decode(&fields); err != nil {
			return fmt.Errorf("text: %w", err)
		}
		*t = rawHeaderFooterText(fields)
		return nil
	default:
		return fmt.Errorf("text: expected a string or a map with left/center/right, got %v", value.Tag)
	}
}

// rawPageNumbersConfig mapea la configuración de numeración de páginas.
// Acepta también un bool (`page_numbers: true`) como atajo de
// `{enabled: true}`, igual que `numbering:` (#100) — no rescata la forma con
// guion (`page-numbers`) que `doclang init`'s `report` template emitía antes
// de #115: esa es una llave equivocada, no una forma alternativa, y
// aliasarla normalizaría el typo en vez de corregirlo.
type rawPageNumbersConfig struct {
	Enabled              bool   `yaml:"enabled"`
	Format               string `yaml:"format"`
	Position             string `yaml:"position"`
	ExcludeTitleSlides   bool   `yaml:"exclude_title_slides"`
	ExcludeClosingSlides bool   `yaml:"exclude_closing_slides"`
	StartFrom            int    `yaml:"start_from"`
	Style                string `yaml:"style"`
}

// rawPageNumbersConfigFields, ver rawHeaderFooterTextFields arriba: mismo
// motivo (tipo definido, no alias, para decodificar el mapa sin recursión).
type rawPageNumbersConfigFields rawPageNumbersConfig

func (p *rawPageNumbersConfig) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		var b bool
		if err := value.Decode(&b); err != nil {
			return fmt.Errorf("page_numbers: expected true/false or a map with 'enabled', got %q", value.Value)
		}
		p.Enabled = b
		return nil
	case yaml.MappingNode:
		var fields rawPageNumbersConfigFields
		if err := value.Decode(&fields); err != nil {
			return fmt.Errorf("page_numbers: %w", err)
		}
		*p = rawPageNumbersConfig(fields)
		return nil
	default:
		return fmt.Errorf("page_numbers: expected true/false or a map with 'enabled', got %v", value.Tag)
	}
}

// rawLogoConfig mapea la configuración de logos
type rawLogoConfig struct {
	Source   string `yaml:"source"`
	Alt      string `yaml:"alt"`
	Height   string `yaml:"height"`
	Position string `yaml:"position"`
}

// rawBorderConfig mapea la configuración de bordes
type rawBorderConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Color    string `yaml:"color"`
	Width    string `yaml:"width"`
	Style    string `yaml:"style"`
	Position string `yaml:"position"`
}

// rawLayoutConfig mapea overrides por layout
type rawLayoutConfig struct {
	Header *rawHeaderConfig `yaml:"header"`
	Footer *rawFooterConfig `yaml:"footer"`
}

func (p *FrontMatterParser) Parse(content string) (*ast.FrontMatterNode, string, []diagnostics.Diagnostic) {
	p.diagnostics = nil

	// Verificar si empieza con ---
	if !strings.HasPrefix(content, FrontMatterDelimiter) {
		return nil, content, []diagnostics.Diagnostic{
			diagnostics.NewError("Missing FrontMatter delimiter",
				diagnostics.NewPosition(1, 1), "parser"),
		}
	}

	// Encontrar el delimitador de cierre
	lines := strings.Split(content, "\n")
	endIndex := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == FrontMatterDelimiter {
			endIndex = i
			break
		}
	}

	if endIndex == -1 {
		return nil, content, []diagnostics.Diagnostic{
			diagnostics.NewError("Missing closing FrontMatter delimiter",
				diagnostics.NewPosition(1, 1), "parser"),
		}
	} // Extraer YAML
	yamlContent := strings.Join(lines[1:endIndex], "\n")
	remainingContent := strings.Join(lines[endIndex+1:], "\n")

	// Parsear YAML
	var raw rawFrontMatter
	if err := yaml.Unmarshal([]byte(yamlContent), &raw); err != nil {
		return nil, content, []diagnostics.Diagnostic{
			diagnostics.NewError(fmt.Sprintf("Invalid YAML: %v", err),
				diagnostics.NewPosition(2, 1), "parser"),
		}
	}
	// Validar campos obligatorios
	if raw.Mode == "" {
		// Si no hay modo especificado, usar auto como fallback
		// Esto es común en contenido generado por AI que se normaliza
		raw.Mode = "auto"
		p.diagnostics = append(p.diagnostics,
			diagnostics.NewWarning("Mode not specified, defaulting to 'auto'",
				diagnostics.NewPosition(2, 1), "parser").WithRuleID("FRONT001"))
	}
	// Validar modos soportados
	// "flex-ai" se mantiene como alias deprecado de "flex-full" (mismo comportamiento,
	// nombre previo antes de dejar de usar branding "AI" para el normalizador determinista)
	validModes := []string{"strict", "flex", "flex-full", "flex-ai", "auto"}
	isValidMode := false
	for _, validMode := range validModes {
		if raw.Mode == validMode {
			isValidMode = true
			break
		}
	}

	if raw.Mode != "" && !isValidMode {
		p.diagnostics = append(p.diagnostics,
			diagnostics.NewError("Invalid mode: must be 'strict', 'flex', 'flex-full', 'flex-ai', or 'auto'",
				diagnostics.NewPosition(2, 1), "parser").WithRuleID("FRONT002"))
	}

	// Crear nodo AST
	node := ast.NewFrontMatterNode(diagnostics.NewPosition(1, 1))
	node.EndPosition = diagnostics.NewPosition(endIndex+1, 4)
	node.Mode = raw.Mode
	node.Title = raw.Title
	node.Author = raw.Author
	node.Date = raw.Date
	node.Theme = raw.Theme
	node.Lang = raw.Lang
	if raw.Numbering != nil {
		node.Numbering = raw.Numbering.Enabled
	}
	node.Variables = raw.Variables
	node.Raw = yamlContent

	// Validar lang (issues #62/#63 code review): sin esto, a11y.IsValidLangTag
	// existía pero nadie lo llamaba, así que un `lang:` malformado ("es_MX",
	// "espanol") llegaba sin aviso hasta <html lang>/w:lang. Warning, no
	// Error como "Invalid mode": a11y.IsValidLangTag valida solo la
	// producción `langtag` de BCP 47 (no `privateuse`/`grandfathered`, ver
	// su doc comment) — un tag real pero fuera de ese subconjunto (p.ej.
	// "x-private") es un falso-rechazo conocido, y un validador con un
	// hueco de cobertura documentado no debe poder tumbar un build (code
	// review de este cambio). "Invalid mode" sí es Error porque rompe el
	// dispatch del parser aguas abajo; un `lang` imperfecto solo degrada
	// metadata de accesibilidad.
	if raw.Lang != "" && !a11y.IsValidLangTag(raw.Lang) {
		p.diagnostics = append(p.diagnostics,
			diagnostics.NewWarning(fmt.Sprintf("Invalid lang: %q is not a well-formed BCP 47 language tag (e.g. \"es\", \"en-US\", \"zh-Hans-CN\")", raw.Lang),
				diagnostics.NewPosition(2, 1), "parser").WithRuleID("FRONT004"))
	}

	// Procesar configuración de headers y footers
	if raw.Header != nil || raw.Footer != nil || raw.LayoutDefaults != nil {
		node.HeaderFooter = p.convertHeaderFooterConfig(&raw)
	}

	return node, remainingContent, p.diagnostics
}

// convertHeaderFooterConfig convierte la configuración raw a estructuras AST
func (p *FrontMatterParser) convertHeaderFooterConfig(raw *rawFrontMatter) *ast.HeaderFooterConfig {
	config := &ast.HeaderFooterConfig{}

	// Convertir header
	if raw.Header != nil {
		config.Header = p.convertHeaderConfig(raw.Header)
	}

	// Convertir footer
	if raw.Footer != nil {
		config.Footer = p.convertFooterConfig(raw.Footer)
	}

	// Convertir layout defaults
	if raw.LayoutDefaults != nil {
		config.LayoutDefaults = make(map[string]*ast.LayoutHeaderFooterConfig)
		for layoutName, layoutConfig := range raw.LayoutDefaults {
			converted := &ast.LayoutHeaderFooterConfig{}

			if layoutConfig.Header != nil {
				converted.Header = p.convertHeaderConfig(layoutConfig.Header)
			}
			if layoutConfig.Footer != nil {
				converted.Footer = p.convertFooterConfig(layoutConfig.Footer)
			}

			config.LayoutDefaults[layoutName] = converted
		}
	}

	return config
}

// convertHeaderConfig convierte configuración de header
func (p *FrontMatterParser) convertHeaderConfig(raw *rawHeaderConfig) *ast.HeaderConfig {
	config := &ast.HeaderConfig{
		Enabled:    raw.Enabled,
		Height:     raw.Height,
		Background: raw.Background,
	}

	if raw.Text != nil {
		config.Text = &ast.HeaderFooterText{
			Left:   raw.Text.Left,
			Center: raw.Text.Center,
			Right:  raw.Text.Right,
		}
	}

	if raw.Logo != nil {
		config.Logo = &ast.LogoConfig{
			Source:   raw.Logo.Source,
			Alt:      raw.Logo.Alt,
			Height:   raw.Logo.Height,
			Position: raw.Logo.Position,
		}
	}

	if raw.Border != nil {
		config.Border = &ast.BorderConfig{
			Enabled:  raw.Border.Enabled,
			Color:    raw.Border.Color,
			Width:    raw.Border.Width,
			Style:    raw.Border.Style,
			Position: raw.Border.Position,
		}
	}

	return config
}

// convertFooterConfig convierte configuración de footer
func (p *FrontMatterParser) convertFooterConfig(raw *rawFooterConfig) *ast.FooterConfig {
	config := &ast.FooterConfig{
		Enabled:    raw.Enabled,
		Height:     raw.Height,
		Background: raw.Background,
	}

	if raw.Text != nil {
		config.Text = &ast.HeaderFooterText{
			Left:   raw.Text.Left,
			Center: raw.Text.Center,
			Right:  raw.Text.Right,
		}
	}

	if raw.PageNumbers != nil {
		config.PageNumbers = &ast.PageNumbersConfig{
			Enabled:              raw.PageNumbers.Enabled,
			Format:               raw.PageNumbers.Format,
			Position:             raw.PageNumbers.Position,
			ExcludeTitleSlides:   raw.PageNumbers.ExcludeTitleSlides,
			ExcludeClosingSlides: raw.PageNumbers.ExcludeClosingSlides,
			StartFrom:            raw.PageNumbers.StartFrom,
			Style:                raw.PageNumbers.Style,
		}
	}

	if raw.Border != nil {
		config.Border = &ast.BorderConfig{
			Enabled:  raw.Border.Enabled,
			Color:    raw.Border.Color,
			Width:    raw.Border.Width,
			Style:    raw.Border.Style,
			Position: raw.Border.Position,
		}
	}

	return config
}
