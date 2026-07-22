// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"sort"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/util"
	"go.ziradocs.com/slidelang/internal/generator/css/themes"
)

type listThemesInput struct{}

// themeSummary es un subconjunto de themes.Theme para el output del tool —
// deliberadamente omite Variables (el mapa completo de CSS custom
// properties, decenas de entradas): un agente eligiendo un tema por nombre
// no lo necesita, y solo infla la respuesta. Es la misma información que ya
// muestra `slidelang themes list` en su tabla NAME/TYPE/VERSION/AUTHOR/DESCRIPTION.
type themeSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Author      string `json:"author"`
	Version     string `json:"version"`
	External    bool   `json:"external"`
}

type listThemesOutput struct {
	Themes []themeSummary `json:"themes"`
}

// registerListThemesTool registra el tool "list_themes": expone los mismos
// temas embebidos + externos que `slidelang themes list`, reusando
// themes.NewThemeLoader().GetAvailableThemes() directamente en vez de
// invocar el subcomando.
func registerListThemesTool(server *sdkmcp.Server, logger util.Logger) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_themes",
		Description: "List available SlideLang themes (embedded and externally installed), for use with the preview tool or the --theme build flag.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in listThemesInput) (*sdkmcp.CallToolResult, listThemesOutput, error) {
		loader := themes.NewThemeLoader()
		available, err := loader.GetAvailableThemes()
		if err != nil {
			res, resErr := errorResult(toolError{Error: err.Error()})
			return res, listThemesOutput{}, resErr
		}

		summaries := make([]themeSummary, 0, len(available))
		for _, t := range available {
			summaries = append(summaries, themeSummary{
				Name:        t.Name,
				Description: t.Description,
				Author:      t.Author,
				Version:     t.Version,
				External:    t.IsExternal,
			})
		}
		sort.Slice(summaries, func(i, j int) bool { return summaries[i].Name < summaries[j].Name })

		out := listThemesOutput{Themes: summaries}
		res, resErr := jsonResult(out)
		return res, out, resErr
	})
}

// validateThemeName rechaza cualquier valor de `theme` que no sea un nombre
// exacto ya conocido por el theme loader (embebido o externo).
//
// name llega acá como input de un tool MCP — a diferencia del flag --theme
// del CLI (siempre operador confiando en su propia terminal), es contenido
// que decide un cliente MCP, potencialmente influenciable por contenido de
// terceros que ese cliente está procesando. generator.resolveTheme trata
// cualquier GeneratorOptions.Theme no vacío como `trusted=true`
// (css/themes/loader.go:93-116) — el mismo nivel de confianza que el flag
// del CLI — lo cual habilita el atajo de path crudo en
// findAndLoadExternalTheme (loader.go:147-151): si el nombre contiene "/",
// "\" o termina en ".json", se hace os.Stat + LoadExternalTheme directo
// sobre ese path, sin confinamiento a los directorios de búsqueda de temas.
// Es la misma clase de vulnerabilidad ya documentada y cerrada para el
// vector de frontmatter (docs/SECURITY_AUDIT_2026-07.md, ME-2) — reabierta acá
// por un vector nuevo si no se valida antes de pasarlo a GeneratorOptions.
// Un nombre vacío (usar el tema del documento/default) siempre es válido.
//
// Usa LoadTheme(name, trusted=false) en vez de GetAvailableThemes(): tiene
// exactamente la misma semántica de confianza que necesitamos acá (rechaza
// cualquier name no-opaco antes de tocar el filesystem, nunca toma el atajo
// de path crudo — ver loader.go:93-116), pero es un solo lookup en vez de
// forzar un discoverExternalThemes() completo (walk de ./themes,
// ~/.slidelang/themes, etc.) en cada llamada — barato para el caso común de
// un nombre embebido ("dark", "default", "minimal": hit directo en el map),
// que antes pagaba el mismo costo que un nombre externo.
func validateThemeName(name string) error {
	if name == "" {
		return nil
	}
	if _, err := themes.NewThemeLoader().LoadTheme(name, false); err != nil {
		return fmt.Errorf("unknown theme %q — call list_themes for the available names: %w", name, err)
	}
	return nil
}
