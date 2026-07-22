// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"go.ziradocs.com/core/util"
	"go.ziradocs.com/doclang/internal/mcp"
)

// NewMCPCommand expone doclang como servidor MCP (Model Context Protocol,
// issue #187/#189) sobre stdio — a la par del de slidelang (issue #133) —
// para que un agente/cliente MCP pueda parsear, lintear, obtener el AST y
// previsualizar DocLang sin invocar `build` por archivo. Configuración
// típica de un cliente MCP:
//
//	{ "command": "doclang", "args": ["mcp"] }
func NewMCPCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start the DocLang MCP server (stdio)",
		Long: `Start an MCP (Model Context Protocol) server over stdio, exposing
DocLang parsing/linting/rendering as tools for an MCP-compatible agent or
client — lint, get_ast, list_themes, preview.

Example client config:
  { "command": "doclang", "args": ["mcp"] }`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			server := mcp.NewServer(util.NewNoop())
			return server.Run(cmd.Context(), &sdkmcp.StdioTransport{})
		},
	}
	return cmd
}
