// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/doclang/v2/internal/mcp"
)

// NewMCPCommand expone doclang como servidor MCP (Model Context Protocol,
// issue #187/#189) sobre stdio — a la par del de slidelang (issue #133) —
// para que un agente/cliente MCP pueda parsear, lintear, obtener el AST y
// previsualizar DocLang sin invocar `build` por archivo. Configuración
// típica de un cliente MCP:
//
//	{ "command": "doclang", "args": ["mcp", "--root", "/path/to/docs"] }
func NewMCPCommand() *cobra.Command {
	var (
		root         string
		chromiumPath string
	)
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start the DocLang MCP server (stdio)",
		Long: `Start an MCP (Model Context Protocol) server over stdio, exposing
DocLang parsing/linting/rendering as tools for an MCP-compatible agent or
client — lint, get_ast, list_themes, preview (in memory, read-only) and
export (writes DOCX, PDF or HTML for a file on disk).

export reads and writes only inside --root (default: the current
directory): the input, the output, @include files and local images. It
never replaces an existing file unless the call passes overwrite: true,
and never downloads Chromium.

Example client config:
  { "command": "doclang", "args": ["mcp", "--root", "/path/to/docs"] }`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			server, err := mcp.NewServerWithOptions(util.NewNoop(), mcp.Options{
				Root:         root,
				ChromiumPath: chromiumPath,
			})
			if err != nil {
				return err
			}
			return server.Run(cmd.Context(), &sdkmcp.StdioTransport{})
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "Directory the export tool is confined to: it reads and writes only inside it (default: the current directory)")
	cmd.Flags().StringVar(&chromiumPath, "chromium-path", "", "Custom path to the Chromium/Chrome/Edge executable used by export (default: auto-detect)")
	return cmd
}
