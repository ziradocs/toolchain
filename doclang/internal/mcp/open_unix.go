// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package mcp

import "syscall"

// openNonBlock evita que abrir un FIFO bloquee (ver resolveInput). En un
// archivo regular O_NONBLOCK no cambia nada.
const openNonBlock = syscall.O_NONBLOCK
