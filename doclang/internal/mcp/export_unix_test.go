// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build unix

package mcp

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// TestExport_FIFOInputDoesNotBlock: un FIFO llamado x.md se rechaza sin
// bloquear. Antes os.Open se quedaba esperando un escritor, antes de que
// empezara el timeout.
func TestExport_FIFOInputDoesNotBlock(t *testing.T) {
	f := newExportFixture(t)
	if err := syscall.Mkfifo(filepath.Join(f.root, "x.md"), 0o644); err != nil {
		t.Skipf("mkfifo: %v", err)
	}
	s, ctx := newExportSession(t, Options{Root: f.root})
	done := make(chan struct{})
	go func() {
		exportFail(t, s, ctx, map[string]any{"path": "x.md", "format": "html"}, "not a regular file")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("export se bloqueó abriendo un FIFO")
	}
}
