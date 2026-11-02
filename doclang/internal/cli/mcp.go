// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"context"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

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
			// Un export en vuelo cuando el cliente cierra stdin, o cuando
			// llega SIGINT/SIGTERM, deja su temporal; se borra antes de
			// salir. La señal cancela el contexto del servidor y Run
			// vuelve. Un kill -9 no pasa por acá.
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sigCh)
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			var received atomic.Value
			go func() {
				select {
				case sig := <-sigCh:
					received.Store(sig)
					cancel()
				case <-ctx.Done():
				}
			}()

			err = server.Run(ctx, &sdkmcp.StdioTransport{})
			mcp.CleanupPendingExports()
			if sig, ok := received.Load().(os.Signal); ok {
				// Salida por señal con el código convencional 128+n
				// (130 para SIGINT, 143 para SIGTERM), como un proceso
				// que no la atrapa. La limpieza ya corrió arriba.
				os.Exit(signalExitCode(sig))
			}
			return err
		},
	}
	cmd.Flags().StringVar(&root, "root", "", "Directory the export tool is confined to: it reads and writes only inside it (default: the current directory)")
	cmd.Flags().StringVar(&chromiumPath, "chromium-path", "", "Custom path to the Chromium/Chrome/Edge executable used by export (default: auto-detect)")
	return cmd
}

// signalExitCode es 128 más el número de la señal, o 1 si no es una
// syscall.Signal.
func signalExitCode(sig os.Signal) int {
	if s, ok := sig.(syscall.Signal); ok {
		return 128 + int(s)
	}
	return 1
}
