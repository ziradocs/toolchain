// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/v2/util"
)

// serverVersion se mantiene independiente de la versión del binario doclang
// (cmd/doclang/main.go) — versiona la superficie de tools MCP, no el CLI.
// Súbela si cambia el input/output shape de un tool existente.
const serverVersion = "0.2.0"

// Options configura el servidor. El valor cero sirve: raíz = directorio de
// trabajo del proceso, Chromium autodetectado, timeout por defecto.
type Options struct {
	// Root es el directorio al que `export` confina lo que lee y escribe
	// (--root de `doclang mcp`). Se resuelve una vez, con
	// filepath.EvalSymlinks, al construir el servidor. Vacío = el
	// directorio de trabajo.
	Root string
	// ChromiumPath fuerza el ejecutable de Chromium/Chrome/Edge
	// (--chromium-path). Vacío = autodetectar.
	ChromiumPath string
	// ExportTimeout acota cada `export`. Cero = DefaultExportTimeout.
	ExportTimeout time.Duration
}

// NewServer construye el servidor MCP de doclang con todos sus tools
// registrados y las Options por defecto. Si el directorio de trabajo no se
// puede resolver, el servidor arranca sin `export` (los otros tools no
// tocan el disco).
func NewServer(logger util.Logger) *sdkmcp.Server {
	server, err := NewServerWithOptions(logger, Options{})
	if err != nil {
		server = newServer(logger)
	}
	return server
}

// NewServerWithOptions es NewServer con Options. logger se inyecta
// explícitamente (patrón G1c) en vez de leer util.GetDefault(): el canal
// stdio de un servidor MCP es el transporte del protocolo — un Noop
// mantiene la salida del proceso limpia por defecto.
func NewServerWithOptions(logger util.Logger, opts Options) (*sdkmcp.Server, error) {
	if logger == nil {
		logger = util.NewNoop()
	}
	root, err := newExportRoot(opts.Root)
	if err != nil {
		return nil, err
	}
	server := newServer(logger)
	registerExportTool(server, logger, exportConfig{
		root:         root,
		chromiumPath: opts.ChromiumPath,
		timeout:      opts.ExportTimeout,
	})
	return server, nil
}

// newServer arma el servidor con los tools que trabajan en memoria.
func newServer(logger util.Logger) *sdkmcp.Server {
	if logger == nil {
		logger = util.NewNoop()
	}

	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "doclang",
		Version: serverVersion,
	}, nil)

	registerLintTool(server, logger)
	registerGetASTTool(server, logger)
	registerListThemesTool(server, logger)
	registerPreviewTool(server, logger)

	return server
}
