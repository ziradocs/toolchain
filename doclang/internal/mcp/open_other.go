// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !unix

package mcp

// openNonBlock no existe fuera de unix; ahí basta con el Stat previo de
// resolveInput.
const openNonBlock = 0
