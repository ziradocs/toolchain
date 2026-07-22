// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"go.ziradocs.com/core/util"
	"go.ziradocs.com/slidelang/internal/mcp"
)

// NewMCPCommand expone slidelang como servidor MCP (Model Context Protocol,
// issue #133) sobre stdio, para que un agente/cliente MCP pueda parsear,
// lintear, obtener el AST y previsualizar SlideLang sin invocar `build` por
// archivo. Configuración típica de un cliente MCP:
//
//	{ "command": "slidelang", "args": ["mcp"] }
func NewMCPCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start the SlideLang MCP server (stdio)",
		Long: `Start an MCP (Model Context Protocol) server over stdio, exposing
SlideLang parsing/linting/rendering as tools for an MCP-compatible agent or
client — lint, get_ast, list_themes, preview.

Example client config:
  { "command": "slidelang", "args": ["mcp"] }`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			server := mcp.NewServer(util.NewNoop())
			return server.Run(cmd.Context(), &sdkmcp.StdioTransport{})
		},
	}
	return cmd
}
