// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"

	"go.ziradocs.com/core/ast"
	"go.ziradocs.com/core/diagnostics"
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
	Variables map[string]interface{} `yaml:"variables"`
	// Configuración de headers y footers
	Header         *rawHeaderConfig            `yaml:"header"`
	Footer         *rawFooterConfig            `yaml:"footer"`
	LayoutDefaults map[string]*rawLayoutConfig `yaml:"layout_defaults"`
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

// rawHeaderFooterText mapea el contenido de texto
type rawHeaderFooterText struct {
	Left   string `yaml:"left"`
	Center string `yaml:"center"`
	Right  string `yaml:"right"`
}

// rawPageNumbersConfig mapea la configuración de numeración
type rawPageNumbersConfig struct {
	Enabled              bool   `yaml:"enabled"`
	Format               string `yaml:"format"`
	Position             string `yaml:"position"`
	ExcludeTitleSlides   bool   `yaml:"exclude_title_slides"`
	ExcludeClosingSlides bool   `yaml:"exclude_closing_slides"`
	StartFrom            int    `yaml:"start_from"`
	Style                string `yaml:"style"`
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
	node.Variables = raw.Variables
	node.Raw = yamlContent

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
