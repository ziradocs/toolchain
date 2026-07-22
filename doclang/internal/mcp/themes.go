// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"fmt"
	"sort"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/util"
	"go.ziradocs.com/doclang/themes/document"
)

type listThemesInput struct{}

// themeSummary es un subconjunto de document.Theme para el output del tool
// — deliberadamente omite Variables (el mapa completo de CSS custom
// properties): un agente eligiendo un tema por nombre no lo necesita.
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
// temas embebidos + externos que resuelve `doclang build --theme`, reusando
// document.NewThemeLoader().ListAvailableThemes() directamente. A diferencia
// del equivalente de slidelang (themes.GetAvailableThemes(), que puede
// fallar), ListAvailableThemes() de doclang no retorna error.
func registerListThemesTool(server *sdkmcp.Server, logger util.Logger) {
	sdkmcp.AddTool(server, &sdkmcp.Tool{
		Name:        "list_themes",
		Description: "List available DocLang themes (embedded and externally installed), for use with the preview tool or the --theme build flag.",
	}, func(ctx context.Context, req *sdkmcp.CallToolRequest, in listThemesInput) (*sdkmcp.CallToolResult, listThemesOutput, error) {
		available := document.NewThemeLoader().ListAvailableThemes()

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
// terceros que ese cliente está procesando. document.ThemeLoader.LoadTheme
// con trusted=false rechaza cualquier name no-opaco (con "/", "\" o "..")
// antes de tocar el filesystem (ver loader.go: mismo mecanismo de
// confinamiento que documenta docs/SECURITY_AUDIT_2026-07.md ME-2 para
// slidelang) — reabierto acá por un vector nuevo si no se valida antes de
// pasarlo a RenderHTMLPreview. Un nombre vacío (usar el tema del
// documento/default) siempre es válido.
func validateThemeName(name string) error {
	if name == "" {
		return nil
	}
	if _, err := document.NewThemeLoader().LoadTheme(name, false); err != nil {
		return fmt.Errorf("unknown theme %q — call list_themes for the available names: %w", name, err)
	}
	return nil
}
