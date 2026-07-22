// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/v2/util"
)

// serverVersion se mantiene independiente de la versión del binario
// slidelang (cmd/slidelang/main.go) — versiona la superficie de tools MCP,
// no el CLI. Súbela si cambia el input/output shape de un tool existente.
const serverVersion = "0.1.0"

// NewServer construye el servidor MCP de slidelang con todos sus tools
// registrados. logger se inyecta explícitamente (patrón G1c) en vez de leer
// util.GetDefault(): el canal stdio de un servidor MCP es el transporte del
// protocolo — un logger real escribiendo a stderr sería inofensivo (ver
// util.ConsoleLogger, issue #46) pero un Noop mantiene la salida del proceso
// limpia por defecto.
func NewServer(logger util.Logger) *sdkmcp.Server {
	if logger == nil {
		logger = util.NewNoop()
	}

	server := sdkmcp.NewServer(&sdkmcp.Implementation{
		Name:    "slidelang",
		Version: serverVersion,
	}, nil)

	registerLintTool(server, logger)
	registerGetASTTool(server, logger)
	registerListThemesTool(server, logger)
	registerPreviewTool(server, logger)

	return server
}
