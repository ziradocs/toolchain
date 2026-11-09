// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build linux && !js

package chromium

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/chromedp/chromedp"
)

// isolateNetwork pone el proceso en un espacio de nombres de usuario y de
// red propios, sin más interfaz que un loopback caído: ninguna conexión
// sale de la máquina, la pida quien la pida. Las banderas de
// noNetworkAllocatorOptions no alcanzan: Chrome y Edge abren conexiones
// directas a una IP fija de DNS sobre HTTPS (2001:4860:4860::8888 en
// Chromium, una de Microsoft en Edge) que no pasan ni por el proxy ni por
// las reglas de resolución. CDP va por pipe (--remote-debugging-pipe), así
// que no necesita red.
func isolateNetwork(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Cloneflags |= syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET
	cmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{{ContainerID: os.Getuid(), HostID: os.Getuid(), Size: 1}}
	cmd.SysProcAttr.GidMappings = []syscall.SysProcIDMap{{ContainerID: os.Getgid(), HostID: os.Getgid(), Size: 1}}
}

// networkIsolation devuelve la opción de arranque que aísla la red de
// Chrome, o un error si este sistema no permite espacios de nombres de
// usuario sin privilegios (se prueba arrancando el mismo binario con
// --version, igual que GetVersion).
func networkIsolation(chromiumPath string) (chromedp.ExecAllocatorOption, error) {
	probe := exec.Command(chromiumPath, "--version")
	isolateNetwork(probe)
	if out, err := probe.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("network namespace not available: %v (%s)", err, firstLine(out))
	}
	return chromedp.ModifyCmdFunc(isolateNetwork), nil
}

func firstLine(b []byte) string {
	for i, c := range b {
		if c == '\n' {
			return string(b[:i])
		}
	}
	return string(b)
}
