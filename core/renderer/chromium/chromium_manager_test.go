// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package chromium

import (
	"os"
	"runtime"
	"testing"
)

// TestChromiumNeedsNoSandbox_EnvOverride es una regresión para
// docs/SECURITY_AUDIT_2026-07.md AL-1: el override explícito debe activarse
// y devolver una razón no vacía para que CreateContext pueda loguear un
// rastro auditable (ver hallazgo de code-review: antes solo se logueaba el
// override, no la detección de root, dejando builds-como-root sin rastro).
func TestChromiumNeedsNoSandbox_EnvOverride(t *testing.T) {
	t.Setenv("CHROMIUM_NO_SANDBOX", "1")

	needsNoSandbox, reason := chromiumNeedsNoSandbox()
	if !needsNoSandbox {
		t.Fatal("expected CHROMIUM_NO_SANDBOX=1 to disable the sandbox")
	}
	if reason == "" {
		t.Error("expected a non-empty reason so the caller can log an audit trail")
	}
}

func TestChromiumNeedsNoSandbox_DefaultsToSandboxed(t *testing.T) {
	t.Setenv("CHROMIUM_NO_SANDBOX", "")

	if os.Geteuid() == 0 {
		t.Skip("running as root in this environment; can't exercise the non-root default here")
	}

	needsNoSandbox, reason := chromiumNeedsNoSandbox()
	if needsNoSandbox {
		t.Errorf("expected the sandbox to stay enabled for a non-root process with no override, got reason: %q", reason)
	}
}

func TestChromiumNeedsNoSandbox_RootIsDetected(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("root/euid semantics do not apply on Windows")
	}
	if os.Geteuid() != 0 {
		t.Skip("this test only runs meaningfully as root (e.g. inside a container); skipping as non-root")
	}

	needsNoSandbox, reason := chromiumNeedsNoSandbox()
	if !needsNoSandbox {
		t.Fatal("expected root to disable the sandbox")
	}
	if reason == "" {
		t.Error("expected a non-empty reason so the caller can log an audit trail")
	}
}
