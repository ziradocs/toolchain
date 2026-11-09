// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build linux && !js

package chromium

import (
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// TestIsolateNetwork: un proceso arrancado con isolateNetwork vive en otro
// espacio de nombres de red. Se salta donde el sistema no permite espacios
// de nombres de usuario sin privilegios (RestrictNetwork cae entonces a las
// banderas, con un aviso).
func TestIsolateNetwork(t *testing.T) {
	cmd := exec.Command("sleep", "5")
	isolateNetwork(cmd)
	if err := cmd.Start(); err != nil {
		t.Skipf("unprivileged network namespaces not available: %v", err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	self, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		t.Skip(err)
	}
	child, err := os.Readlink(fmt.Sprintf("/proc/%d/ns/net", cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	if child == self {
		t.Fatalf("isolated process shares the network namespace %s", self)
	}
}
