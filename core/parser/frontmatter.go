// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"fmt"
	"math"
	"strings"

	"go.yaml.in/yaml/v3"

	"go.ziradocs.com/core/v2/a11y"
	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/util"
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
	TOC       *rawTOC                `yaml:"toc"`
	Page      *rawPage               `yaml:"page"`
	Watermark *rawWatermark          `yaml:"watermark"`
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

// rawTOC and rawPage (below) both back the `toc:`/`page:` front matter
// namespaces. Unlike every raw* type above — whose UnmarshalYAML methods
// return an error for a bad shape, which yaml.Unmarshal propagates as a
// hard failure of the ENTIRE front matter (see Parse's `yaml.Unmarshal`
// call) — these two never return an error. `numbering:`'s bad-shape forms
// were enumerable because doclang init emitted all of them; `toc:`/`page:`
// are open keys nobody enumerated (they were "known-ignored" until now,
// see frontmatter.md), so a form nobody anticipated must degrade to a
// warning, not take the rest of a valid document (title, author, header/
// footer...) down with it — the exact failure mode issue #115 was about,
// just one level up (a wrong VALUE instead of a wrong KEY).
//
// Every field is a raw *yaml.Node rather than a concrete bool/int/string:
// decoding into *yaml.Node cannot fail, so no sub-value (a quoted
// `depth: "3"`, an out-of-range depth, an unrecognized `size:`) can abort
// UnmarshalYAML either. convertTOC/convertPage — the only place with
// diagnostics access — do the actual decode-to-concrete-type and semantic
// validation (via core/util's shared length/paper-size resolver), and
// downgrade any failure there to a FRONT005/FRONT006 warning with the
// field just left unset.
//
// The MappingNode branch walks value.Content pairs by hand instead of
// `value.Decode(&fields)` into a struct with `yaml:"enabled"`/`yaml:"depth"`
// tags — a struct decode silently drops any key it doesn't recognize (code
// review finding on the first version of this type: `toc: {enable: false}`,
// a one-letter typo for "enabled", decoded to an all-nil rawTOC with no
// diagnostic at all, the exact silent-drop failure mode issue #121 tracks).
// Walking Content directly means every key is seen, so a typo can be
// collected into unknownKeys and surfaced as a warning by convertTOC.
type rawTOC struct {
	Enabled *yaml.Node
	Depth   *yaml.Node
	// unknownKeys holds any `toc:` map key other than "enabled"/"depth"
	// (most commonly a typo) — reported as a FRONT005 warning by convertTOC
	// rather than silently dropped.
	unknownKeys []string
	// duplicateKeys holds any known key ("enabled"/"depth") that appears
	// more than once in the map — YAML itself allows repeated keys, and the
	// last one silently wins the assignment above with no diagnostic unless
	// this is tracked (code review finding: `toc: {enabled: true, enabled:
	// false}` decoded to `false` with zero warnings).
	duplicateKeys []string
	// badShape holds the offending Tag when `toc:` itself is neither a
	// scalar nor a map (e.g. a YAML sequence) — recorded instead of
	// returned as an error, per this type's contract above.
	badShape string
}

func (t *rawTOC) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		// `toc: true` / `toc: false` — the shape `doclang init`'s
		// examples use today. Stored as the enabled node directly; a
		// non-bool scalar (e.g. `toc: sí`) is caught later in convertTOC.
		t.Enabled = value
		return nil
	case yaml.MappingNode:
		// value.Content alternates key, value, key, value... for a
		// MappingNode; each key is itself a ScalarNode whose Value is the
		// map key's text.
		for i := 0; i+1 < len(value.Content); i += 2 {
			key, val := value.Content[i], value.Content[i+1]
			switch key.Value {
			case "enabled":
				if t.Enabled != nil {
					t.duplicateKeys = append(t.duplicateKeys, key.Value)
				}
				t.Enabled = val
			case "depth":
				if t.Depth != nil {
					t.duplicateKeys = append(t.duplicateKeys, key.Value)
				}
				t.Depth = val
			default:
				t.unknownKeys = append(t.unknownKeys, key.Value)
			}
		}
		return nil
	default:
		t.badShape = value.Tag
		return nil
	}
}

// rawPage backs `page:`. See rawTOC's doc comment above for the shared
// "value never aborts Parse" contract both types follow, and for why the
// MappingNode branch walks Content by hand instead of decoding into a
// tagged struct.
type rawPage struct {
	Size    *yaml.Node
	Margins *yaml.Node
	// unknownKeys holds any `page:` map key other than "size"/"margins"
	// (e.g. the singular typo "margin") — reported as a FRONT006 warning by
	// convertPage.
	unknownKeys []string
	// duplicateKeys holds any known key ("size"/"margins") repeated in the
	// map — see rawTOC.duplicateKeys' doc comment for why this is tracked.
	duplicateKeys []string
	badShape      string
}

func (p *rawPage) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		// `page: A4` — a shorthand for `page: {size: A4}`. Necessary even
		// though no known template emits it: without it, `page: A4`
		// (which parsed fine as an unknown key before this PR) would go
		// from inert to a hard parse failure the moment `page:` becomes a
		// known key, for a form that reads as entirely reasonable to try.
		p.Size = value
		return nil
	case yaml.MappingNode:
		for i := 0; i+1 < len(value.Content); i += 2 {
			key, val := value.Content[i], value.Content[i+1]
			switch key.Value {
			case "size":
				if p.Size != nil {
					p.duplicateKeys = append(p.duplicateKeys, key.Value)
				}
				p.Size = val
			case "margins":
				if p.Margins != nil {
					p.duplicateKeys = append(p.duplicateKeys, key.Value)
				}
				p.Margins = val
			default:
				p.unknownKeys = append(p.unknownKeys, key.Value)
			}
		}
		return nil
	default:
		p.badShape = value.Tag
		return nil
	}
}

// rawWatermark backs `watermark:` (issue #179). See rawTOC's doc comment
// for the shared "value never aborts Parse" contract both types follow,
// and for why the MappingNode branch walks Content by hand instead of
// decoding into a tagged struct — a typo in any of these seven keys must
// surface as a warning, not decode to a silently-incomplete watermark.
type rawWatermark struct {
	Enabled  *yaml.Node
	Text     *yaml.Node
	Color    *yaml.Node
	Opacity  *yaml.Node
	Rotation *yaml.Node
	FontSize *yaml.Node
	Repeat   *yaml.Node
	// unknownKeys holds any `watermark:` map key other than the seven
	// recognized ones — reported as a FRONT007 warning by convertWatermark.
	unknownKeys []string
	// duplicateKeys holds any known key repeated in the map — see
	// rawTOC.duplicateKeys' doc comment for why this is tracked.
	duplicateKeys []string
	badShape      string
}

func (w *rawWatermark) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		// `watermark: "BORRADOR"` — shorthand for
		// `{enabled: true, text: "BORRADOR"}`, the same "the interesting
		// value is what a scalar means" pattern as `page: A4`.
		w.Text = value
		return nil
	case yaml.MappingNode:
		for i := 0; i+1 < len(value.Content); i += 2 {
			key, val := value.Content[i], value.Content[i+1]
			switch key.Value {
			case "enabled":
				if w.Enabled != nil {
					w.duplicateKeys = append(w.duplicateKeys, key.Value)
				}
				w.Enabled = val
			case "text":
				if w.Text != nil {
					w.duplicateKeys = append(w.duplicateKeys, key.Value)
				}
				w.Text = val
			case "color":
				if w.Color != nil {
					w.duplicateKeys = append(w.duplicateKeys, key.Value)
				}
				w.Color = val
			case "opacity":
				if w.Opacity != nil {
					w.duplicateKeys = append(w.duplicateKeys, key.Value)
				}
				w.Opacity = val
			case "rotation":
				if w.Rotation != nil {
					w.duplicateKeys = append(w.duplicateKeys, key.Value)
				}
				w.Rotation = val
			case "font_size":
				if w.FontSize != nil {
					w.duplicateKeys = append(w.duplicateKeys, key.Value)
				}
				w.FontSize = val
			case "repeat":
				if w.Repeat != nil {
					w.duplicateKeys = append(w.duplicateKeys, key.Value)
				}
				w.Repeat = val
			default:
				w.unknownKeys = append(w.unknownKeys, key.Value)
			}
		}
		return nil
	default:
		w.badShape = value.Tag
		return nil
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
	node.TOC = p.convertTOC(raw.TOC)
	node.Page = p.convertPage(raw.Page)
	node.Watermark = p.convertWatermark(raw.Watermark)
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

// tocFrontMatterWarning appends a FRONT005 warning — see rawTOC's doc
// comment for why a bad `toc:` value degrades to a warning instead of
// failing Parse.
func (p *FrontMatterParser) tocFrontMatterWarning(message string) {
	p.diagnostics = append(p.diagnostics,
		diagnostics.NewWarning(message, diagnostics.NewPosition(2, 1), "parser").WithRuleID("FRONT005"))
}

// pageFrontMatterWarning is tocFrontMatterWarning's FRONT006 counterpart
// for `page:`.
func (p *FrontMatterParser) pageFrontMatterWarning(message string) {
	p.diagnostics = append(p.diagnostics,
		diagnostics.NewWarning(message, diagnostics.NewPosition(2, 1), "parser").WithRuleID("FRONT006"))
}

// convertTOC convierte rawTOC a ast.TOCConfig. nil en cualquier salida
// temprana (raw nil, forma inválida, o un mapa que no dejó ninguna
// información — mismo criterio que numbering: {}) es intencional: node.TOC
// nil es "el documento no tiene opinión", el mismo tri-estado que
// node.Numbering.
func (p *FrontMatterParser) convertTOC(raw *rawTOC) *ast.TOCConfig {
	if raw == nil {
		return nil
	}
	if raw.badShape != "" {
		p.tocFrontMatterWarning(fmt.Sprintf("Invalid 'toc:' value (%s); expected true/false or a map with 'enabled'/'depth' — ignored", raw.badShape))
		return nil
	}
	if len(raw.unknownKeys) > 0 {
		p.tocFrontMatterWarning(fmt.Sprintf("Unrecognized key(s) in 'toc:' (%s); expected 'enabled'/'depth' — ignored", strings.Join(raw.unknownKeys, ", ")))
	}
	if len(raw.duplicateKeys) > 0 {
		p.tocFrontMatterWarning(fmt.Sprintf("Duplicate key(s) in 'toc:' (%s); the last occurrence wins", strings.Join(raw.duplicateKeys, ", ")))
	}

	config := &ast.TOCConfig{}

	if raw.Enabled != nil {
		var enabled bool
		if err := raw.Enabled.Decode(&enabled); err != nil {
			p.tocFrontMatterWarning(fmt.Sprintf("Invalid 'toc.enabled': expected true/false, got %q — ignored", raw.Enabled.Value))
		} else {
			config.Enabled = &enabled
		}
	}

	if raw.Depth != nil {
		var depth int
		if err := raw.Depth.Decode(&depth); err != nil {
			p.tocFrontMatterWarning(fmt.Sprintf("Invalid 'toc.depth': expected a number, got %q — ignored", raw.Depth.Value))
		} else if depth < 1 || depth > 6 {
			p.tocFrontMatterWarning(fmt.Sprintf("'toc.depth': %d is outside the usual 1-6 heading range — kept as-is", depth))
			config.Depth = &depth
		} else {
			config.Depth = &depth
		}
	}

	if config.Enabled == nil && config.Depth == nil {
		return nil
	}
	return config
}

// convertPage convierte rawPage a ast.PageConfig. Mismo criterio de nil que
// convertTOC. Size/Margins se guardan tal cual los escribió el autor
// ("A4", "2cm") — core/util.PaperSizeInches/ParseLengthInches se usan acá
// SOLO para validar (avisar si el valor no resuelve a nada conocido), nunca
// para reescribir el campo: el AST es el contrato, y quien lo consume
// decide cómo resolver unidades (ver core/util/length.go).
func (p *FrontMatterParser) convertPage(raw *rawPage) *ast.PageConfig {
	if raw == nil {
		return nil
	}
	if raw.badShape != "" {
		p.pageFrontMatterWarning(fmt.Sprintf("Invalid 'page:' value (%s); expected a size string or a map with 'size'/'margins' — ignored", raw.badShape))
		return nil
	}
	if len(raw.unknownKeys) > 0 {
		p.pageFrontMatterWarning(fmt.Sprintf("Unrecognized key(s) in 'page:' (%s); expected 'size'/'margins' — ignored", strings.Join(raw.unknownKeys, ", ")))
	}
	if len(raw.duplicateKeys) > 0 {
		p.pageFrontMatterWarning(fmt.Sprintf("Duplicate key(s) in 'page:' (%s); the last occurrence wins", strings.Join(raw.duplicateKeys, ", ")))
	}

	config := &ast.PageConfig{}

	if raw.Size != nil {
		var size string
		if err := raw.Size.Decode(&size); err != nil {
			p.pageFrontMatterWarning(fmt.Sprintf("Invalid 'page.size': expected a string, got %v — ignored", raw.Size.Tag))
		} else {
			if _, _, ok := util.PaperSizeInches(size); !ok {
				p.pageFrontMatterWarning(fmt.Sprintf("'page.size': %q is not a recognized paper size (e.g. A4, Letter, Legal) — kept as-is", size))
			}
			config.Size = size
		}
	}

	if raw.Margins != nil {
		config.Margins = p.convertPageMargins(raw.Margins)
	}

	if config.Size == "" && config.Margins == nil {
		return nil
	}
	return config
}

// convertPageMargins decodifica `page.margins`, que acepta un escalar
// (llena los 4 lados) o un mapa por lado — mismo tipo de atajo escalar que
// rawHeaderFooterText.Center/rawPageNumbersConfig.Enabled arriba, pero
// resuelto acá directamente sobre el *yaml.Node en vez de un raw type
// propio con su propio UnmarshalYAML: convertPage ya tiene el nodo en
// mano, y esto necesita el mismo acceso a p.diagnostics que el resto de
// este archivo, no una decodificación aislada.
//
// El MappingNode camina node.Content a mano (mismo motivo que
// rawTOC/rawPage.UnmarshalYAML, ver su doc comment): decodificar a un
// struct con `yaml:"top"`/etc descartaba en silencio cualquier llave no
// reconocida (`page.margins.topp` por typo nunca generaba FRONT006).
func (p *FrontMatterParser) convertPageMargins(node *yaml.Node) *ast.PageMargins {
	validateSide := func(label, value string) {
		if value == "" {
			return
		}
		if _, err := util.ParseLengthInches(value); err != nil {
			p.pageFrontMatterWarning(fmt.Sprintf("'page.margins.%s': %q is not a recognized length (e.g. 2cm, 0.5in, 40px) — kept as-is", label, value))
		}
	}

	switch node.Kind {
	case yaml.ScalarNode:
		var s string
		if err := node.Decode(&s); err != nil {
			p.pageFrontMatterWarning(fmt.Sprintf("Invalid 'page.margins': expected a string or a map with top/right/bottom/left, got %q — ignored", node.Value))
			return nil
		}
		if s == "" {
			return nil
		}
		validateSide("(all sides)", s)
		return &ast.PageMargins{Top: s, Right: s, Bottom: s, Left: s}
	case yaml.MappingNode:
		var sides ast.PageMargins
		var unknownKeys []string
		// duplicateKeys/seen: same rationale as rawTOC/rawPage's
		// duplicateKeys field above — YAML allows a repeated map key, and
		// the last one silently wins the field assignment below unless
		// this is tracked (`page.margins: {top: 1in, top: 2in}` decoded to
		// "2in" with zero diagnostics before this).
		var duplicateKeys []string
		seen := make(map[string]bool, 4)
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, val := node.Content[i], node.Content[i+1]
			var v string
			if err := val.Decode(&v); err != nil {
				p.pageFrontMatterWarning(fmt.Sprintf("'page.margins.%s': expected a string, got %v — ignored", key.Value, val.Tag))
				continue
			}
			switch key.Value {
			case "top", "right", "bottom", "left":
				if seen[key.Value] {
					duplicateKeys = append(duplicateKeys, key.Value)
				}
				seen[key.Value] = true
			}
			switch key.Value {
			case "top":
				sides.Top = v
			case "right":
				sides.Right = v
			case "bottom":
				sides.Bottom = v
			case "left":
				sides.Left = v
			default:
				unknownKeys = append(unknownKeys, key.Value)
			}
		}
		if len(unknownKeys) > 0 {
			p.pageFrontMatterWarning(fmt.Sprintf("Unrecognized key(s) in 'page.margins' (%s); expected 'top'/'right'/'bottom'/'left' — ignored", strings.Join(unknownKeys, ", ")))
		}
		if len(duplicateKeys) > 0 {
			p.pageFrontMatterWarning(fmt.Sprintf("Duplicate key(s) in 'page.margins' (%s); the last occurrence wins", strings.Join(duplicateKeys, ", ")))
		}
		if sides.Top == "" && sides.Right == "" && sides.Bottom == "" && sides.Left == "" {
			return nil
		}
		validateSide("top", sides.Top)
		validateSide("right", sides.Right)
		validateSide("bottom", sides.Bottom)
		validateSide("left", sides.Left)
		return &sides
	default:
		p.pageFrontMatterWarning(fmt.Sprintf("Invalid 'page.margins': expected a string or a map with top/right/bottom/left, got %v — ignored", node.Tag))
		return nil
	}
}

// Defaults applied by convertWatermark when `watermark:` is present but a
// given key isn't — chosen to match the "subtle diagonal repeating text"
// visual the feature exists for (issue #179), not a bold/opaque stamp.
const (
	defaultWatermarkOpacity  = 0.08
	defaultWatermarkRotation = -45.0
	defaultWatermarkColor    = "#000000"
	defaultWatermarkFontSize = "72pt"
)

// watermarkFrontMatterWarning is tocFrontMatterWarning's FRONT007
// counterpart for `watermark:`.
func (p *FrontMatterParser) watermarkFrontMatterWarning(message string) {
	p.diagnostics = append(p.diagnostics,
		diagnostics.NewWarning(message, diagnostics.NewPosition(2, 1), "parser").WithRuleID("FRONT007"))
}

// convertWatermark convierte rawWatermark a ast.WatermarkConfig. A
// diferencia de convertTOC/convertPage, nil solo se devuelve si raw es nil
// o su forma es inválida — no hay "documento sin opinión" a medias acá:
// declarar `watermark:` en cualquier forma (incluido el atajo escalar) es
// en sí mismo la señal de intención, así que Enabled arranca en true y solo
// un `enabled: false` explícito lo apaga. Esto es deliberadamente distinto
// del bool plano de HeaderConfig.Enabled (que exige `enabled: true`
// explícito) — ahí "declarado" y "encendido" son cosas separadas porque un
// documento puede declarar header/footer con overrides por-layout sin
// querer que el global se dibuje; acá no existe ese caso de uso.
func (p *FrontMatterParser) convertWatermark(raw *rawWatermark) *ast.WatermarkConfig {
	if raw == nil {
		return nil
	}
	if raw.badShape != "" {
		p.watermarkFrontMatterWarning(fmt.Sprintf("Invalid 'watermark:' value (%s); expected a string or a map with 'enabled'/'text'/'color'/'opacity'/'rotation'/'font_size'/'repeat' — ignored", raw.badShape))
		return nil
	}
	if len(raw.unknownKeys) > 0 {
		p.watermarkFrontMatterWarning(fmt.Sprintf("Unrecognized key(s) in 'watermark:' (%s); expected 'enabled'/'text'/'color'/'opacity'/'rotation'/'font_size'/'repeat' — ignored", strings.Join(raw.unknownKeys, ", ")))
	}
	if len(raw.duplicateKeys) > 0 {
		p.watermarkFrontMatterWarning(fmt.Sprintf("Duplicate key(s) in 'watermark:' (%s); the last occurrence wins", strings.Join(raw.duplicateKeys, ", ")))
	}

	config := &ast.WatermarkConfig{Enabled: true}

	if raw.Enabled != nil {
		var enabled bool
		if err := raw.Enabled.Decode(&enabled); err != nil {
			p.watermarkFrontMatterWarning(fmt.Sprintf("Invalid 'watermark.enabled': expected true/false, got %q — ignored", raw.Enabled.Value))
		} else {
			config.Enabled = enabled
		}
	}

	if raw.Text != nil {
		var text string
		if err := raw.Text.Decode(&text); err != nil {
			p.watermarkFrontMatterWarning(fmt.Sprintf("Invalid 'watermark.text': expected a string, got %v — ignored", raw.Text.Tag))
		} else {
			config.Text = text
		}
	}

	if raw.Color != nil {
		var color string
		if err := raw.Color.Decode(&color); err != nil {
			p.watermarkFrontMatterWarning(fmt.Sprintf("Invalid 'watermark.color': expected a string, got %v — ignored", raw.Color.Tag))
		} else {
			if _, _, _, ok := a11y.ParseColor(color); !ok {
				p.watermarkFrontMatterWarning(fmt.Sprintf("'watermark.color': %q is not a recognized CSS color — kept as-is", color))
			}
			config.Color = color
		}
	}

	if raw.Opacity != nil {
		var opacity float64
		if err := raw.Opacity.Decode(&opacity); err != nil {
			p.watermarkFrontMatterWarning(fmt.Sprintf("Invalid 'watermark.opacity': expected a number, got %q — ignored", raw.Opacity.Value))
		} else if math.IsNaN(opacity) || math.IsInf(opacity, 0) {
			p.watermarkFrontMatterWarning(fmt.Sprintf("Invalid 'watermark.opacity': %q must be finite — ignored", raw.Opacity.Value))
		} else {
			clamped := math.Min(1, math.Max(0, opacity))
			if clamped != opacity {
				p.watermarkFrontMatterWarning(fmt.Sprintf("'watermark.opacity': %v is outside 0.0-1.0 — clamped to %v", opacity, clamped))
			}
			config.Opacity = &clamped
		}
	}

	if raw.Rotation != nil {
		var rotation float64
		if err := raw.Rotation.Decode(&rotation); err != nil {
			p.watermarkFrontMatterWarning(fmt.Sprintf("Invalid 'watermark.rotation': expected a number, got %q — ignored", raw.Rotation.Value))
		} else if math.IsNaN(rotation) || math.IsInf(rotation, 0) {
			p.watermarkFrontMatterWarning(fmt.Sprintf("Invalid 'watermark.rotation': %q must be finite — ignored", raw.Rotation.Value))
		} else {
			normalized := math.Mod(rotation, 360)
			config.Rotation = &normalized
		}
	}

	if raw.FontSize != nil {
		var fontSize string
		if err := raw.FontSize.Decode(&fontSize); err != nil {
			p.watermarkFrontMatterWarning(fmt.Sprintf("Invalid 'watermark.font_size': expected a string, got %v — ignored", raw.FontSize.Tag))
		} else {
			if _, err := util.ParseLengthInches(fontSize); err != nil {
				p.watermarkFrontMatterWarning(fmt.Sprintf("'watermark.font_size': %q is not a recognized length (e.g. 72pt, 2cm, 40px) — kept as-is", fontSize))
			}
			config.FontSize = fontSize
		}
	}

	if raw.Repeat != nil {
		var repeat bool
		if err := raw.Repeat.Decode(&repeat); err != nil {
			p.watermarkFrontMatterWarning(fmt.Sprintf("Invalid 'watermark.repeat': expected true/false, got %q — ignored", raw.Repeat.Value))
		} else {
			config.Repeat = &repeat
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
