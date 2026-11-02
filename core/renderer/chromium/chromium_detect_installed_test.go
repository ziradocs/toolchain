// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// fakeHome apunta HOME (y USERPROFILE, que es lo que lee os.UserHomeDir en
// Windows) a un directorio temporal, para que installDirPath caiga ahí.
func fakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

// fakeInstall deja un ejecutable falso donde InstallChromium deja el
// headless shell, usando las mismas funciones que el instalador, y devuelve
// su ruta y el directorio de extracción. withMarker escribe la marca de
// instalación completa.
func fakeInstall(t *testing.T, brand ChromiumBrand, withMarker bool) (execPath, extractDir string) {
	t.Helper()
	if _, ok := chromiumSHA256[chromiumPlatformKey()]; !ok {
		t.Skipf("plataforma %s sin hash fijado: --install-chromium no aplica", chromiumPlatformKey())
	}
	installDir, err := newChromiumInstaller(noopChromiumLogger{}, brand).installDirPath()
	if err != nil {
		t.Fatal(err)
	}
	rel, err := headlessShellRelPath()
	if err != nil {
		t.Skip(err)
	}
	extractDir = installExtractDir(installDir)
	execPath = filepath.Join(extractDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(execPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if withMarker {
		if err := writeInstallMarker(extractDir); err != nil {
			t.Fatal(err)
		}
	}
	return execPath, extractDir
}

// TestDetect_FindsInstalledHeadlessShell: lo que deja --install-chromium se
// encuentra en la siguiente corrida sin --chromium-path ni --install-chromium.
// Antes Detect miraba ~/.<brand>/chromium/chrome, una ruta que nada escribía.
func TestDetect_FindsInstalledHeadlessShell(t *testing.T) {
	fakeHome(t)
	execPath, _ := fakeInstall(t, DefaultChromiumBrand, true)

	got, err := NewChromiumManager("", false, noopChromiumLogger{}).Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if got != execPath {
		t.Errorf("Detect() = %q, want %q", got, execPath)
	}
}

// TestDetect_ExplicitPathWinsOverInstalled: --chromium-path sigue primero.
func TestDetect_ExplicitPathWinsOverInstalled(t *testing.T) {
	fakeHome(t)
	fakeInstall(t, DefaultChromiumBrand, true)
	custom := filepath.Join(t.TempDir(), "my-chrome")
	if err := os.WriteFile(custom, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := NewChromiumManager(custom, false, noopChromiumLogger{}).Detect()
	if err != nil || got != custom {
		t.Errorf("Detect() = %q, %v; want %q", got, err, custom)
	}
}

// TestDetect_UsesOwnBrandCache: slidelang instala bajo su propio brand y lo
// encuentra ahí; no toma la instalación de otro brand.
func TestDetect_UsesOwnBrandCache(t *testing.T) {
	fakeHome(t)
	other := ChromiumBrand{Name: "slidelang"}
	execPath, _ := fakeInstall(t, other, true)

	got, _ := newChromiumManager("", false, noopChromiumLogger{}, other).Detect()
	if got != execPath {
		t.Errorf("Detect() con brand %q = %q, want %q", other.Name, got, execPath)
	}
	if got, _ := NewChromiumManager("", false, noopChromiumLogger{}).Detect(); got == execPath {
		t.Errorf("el brand por defecto tomó la instalación de %q", other.Name)
	}
}

// TestIsChromiumInstalled_RejectsIncompleteOrStale: sin marca (descarga o
// extracción interrumpida), con la marca de otra versión, o con el binario
// sin permiso de ejecución, la instalación no se usa.
func TestIsChromiumInstalled_RejectsIncompleteOrStale(t *testing.T) {
	cases := map[string]func(t *testing.T, execPath, extractDir string){
		"sin marca": func(t *testing.T, _, _ string) {},
		"otra versión": func(t *testing.T, _, extractDir string) {
			stale := "version=0.0.0.0\nplatform=" + chromiumPlatformKey() + "\nsha256=" + chromiumSHA256[chromiumPlatformKey()] + "\n"
			if err := os.WriteFile(filepath.Join(extractDir, installMarkerName), []byte(stale), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"otro hash": func(t *testing.T, _, extractDir string) {
			stale := "version=" + chromiumVersion + "\nplatform=" + chromiumPlatformKey() + "\nsha256=deadbeef\n"
			if err := os.WriteFile(filepath.Join(extractDir, installMarkerName), []byte(stale), 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"marca vacía": func(t *testing.T, _, extractDir string) {
			if err := os.WriteFile(filepath.Join(extractDir, installMarkerName), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		},
		"no ejecutable": func(t *testing.T, execPath, extractDir string) {
			if runtime.GOOS == "windows" {
				t.Skip("sin bits de ejecución en Windows")
			}
			if err := writeInstallMarker(extractDir); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(execPath, 0o644); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			fakeHome(t)
			execPath, extractDir := fakeInstall(t, DefaultChromiumBrand, false)
			setup(t, execPath, extractDir)

			if ok, path := NewChromiumInstaller(noopChromiumLogger{}).IsChromiumInstalled(); ok {
				t.Errorf("IsChromiumInstalled() = true, %q; want false", path)
			}
			if got, _ := NewChromiumManager("", false, noopChromiumLogger{}).Detect(); got == execPath {
				t.Errorf("Detect() devolvió la instalación incompleta %q", got)
			}
		})
	}
}

// TestDetect_DoesNotCreateInstallDir: detectar no crea el directorio de
// cache; eso solo lo hace InstallChromium.
func TestDetect_DoesNotCreateInstallDir(t *testing.T) {
	fakeHome(t)
	installDir, err := NewChromiumInstaller(noopChromiumLogger{}).installDirPath()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = NewChromiumManager("", false, noopChromiumLogger{}).Detect()
	if _, err := os.Stat(installDir); !os.IsNotExist(err) {
		t.Errorf("Detect() creó %s (stat err = %v)", installDir, err)
	}
}
