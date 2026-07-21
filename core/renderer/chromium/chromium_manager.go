// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/chromedp/chromedp"
)

// ChromiumManager gestiona la detección, descarga e instalación de Chromium
type ChromiumManager struct {
	chromiumPath string
	autoInstall  bool
	logger       ChromiumLogger
	brand        ChromiumBrand
}

// ChromiumLogger interface para logging en chromium components
type ChromiumLogger interface {
	Info(tag, format string, args ...interface{})
	Warn(tag, format string, args ...interface{})
	Error(tag, format string, args ...interface{})
}

// NewChromiumManager crea un nuevo gestor de Chromium con el branding por
// defecto de doclang (issue #92). Ver newChromiumManager para inyectar un
// ChromiumBrand distinto.
func NewChromiumManager(customPath string, autoInstall bool, logger ChromiumLogger) *ChromiumManager {
	return newChromiumManager(customPath, autoInstall, logger, DefaultChromiumBrand)
}

// newChromiumManager es la construcción real; brand se inyecta por instancia
// en vez de leerse de estado global (issue #92). Sin exportar: solo
// NewChromiumRendererWithBrand necesita pasar un brand distinto al default, y
// lo hace a través del renderer, no directamente sobre el manager.
func newChromiumManager(customPath string, autoInstall bool, logger ChromiumLogger, brand ChromiumBrand) *ChromiumManager {
	return &ChromiumManager{
		chromiumPath: customPath,
		autoInstall:  autoInstall,
		logger:       logger,
		brand:        brand,
	}
}

// Detect intenta detectar un navegador Chromium instalado
// Retorna la ruta si encuentra uno, error si no
func (m *ChromiumManager) Detect() (string, error) {
	// 1. Si el usuario especificó un path, usarlo
	if m.chromiumPath != "" {
		if m.isValidChromium(m.chromiumPath) {
			m.logger.Info("CHROMIUM", "Using custom Chromium: %s", m.chromiumPath)
			return m.chromiumPath, nil
		}
		return "", fmt.Errorf("invalid chromium path: %s", m.chromiumPath)
	}

	// 2. Intentar detectar automáticamente
	m.logger.Info("CHROMIUM", "Auto-detecting Chromium installation...")

	paths := m.getCommonChromiumPaths()
	for _, path := range paths {
		if m.isValidChromium(path) {
			m.logger.Info("CHROMIUM", "✅ Found Chromium at: %s", path)
			return path, nil
		}
	}

	// 3. Revisar en PATH del sistema
	if path, err := exec.LookPath("chromium"); err == nil {
		m.logger.Info("CHROMIUM", "✅ Found Chromium in PATH: %s", path)
		return path, nil
	}
	if path, err := exec.LookPath("chromium-browser"); err == nil {
		m.logger.Info("CHROMIUM", "✅ Found Chromium in PATH: %s", path)
		return path, nil
	}
	if path, err := exec.LookPath("google-chrome"); err == nil {
		m.logger.Info("CHROMIUM", "✅ Found Chrome in PATH: %s", path)
		return path, nil
	}
	if path, err := exec.LookPath("microsoft-edge"); err == nil {
		m.logger.Info("CHROMIUM", "✅ Found Edge in PATH: %s", path)
		return path, nil
	}
	if path, err := exec.LookPath("msedge"); err == nil {
		m.logger.Info("CHROMIUM", "✅ Found Edge in PATH: %s", path)
		return path, nil
	}

	// 4. Revisar instalación local de doclang
	localPath := m.getLocalChromiumPath()
	if m.isValidChromium(localPath) {
		m.logger.Info("CHROMIUM", "✅ Found local Chromium: %s", localPath)
		return localPath, nil
	}

	// 5. No encontrado - retornar error con hint
	return "", fmt.Errorf("chromium not found. Install Chrome/Chromium/Edge or use --install-chromium flag")
}

// EnsureChromium garantiza que Chromium esté disponible
// Si no lo encuentra y autoInstall=true, lo descarga automáticamente. ctx
// acota/cancela la descarga (issue #134/G1d).
func (m *ChromiumManager) EnsureChromium(ctx context.Context) (string, error) {
	// Intentar detectar primero
	path, err := m.Detect()
	if err == nil {
		return path, nil
	}

	// Si no encontró y autoInstall está habilitado, descargar
	if m.autoInstall {
		m.logger.Info("CHROMIUM", "Chromium not found. Downloading...")
		return m.downloadChromium(ctx)
	}

	// No encontrado y sin auto-install
	return "", fmt.Errorf(`%w

💡 Solutions:
  1. Install Google Chrome, Microsoft Edge, or Chromium browser
  2. Use --install-chromium flag to download automatically
  3. Specify path with --chromium-path=/path/to/chromium

%s`, err, m.brand.InstallHint)
}

// downloadChromium descarga Chromium automáticamente
func (m *ChromiumManager) downloadChromium(ctx context.Context) (string, error) {
	// Usar el instalador automático, con el mismo brand que este manager.
	installer := newChromiumInstaller(m.logger, m.brand)

	// Verificar si ya está instalado
	if installed, path := installer.IsChromiumInstalled(); installed {
		m.logger.Info("CHROMIUM", "Chromium already installed at: %s", path)
		return path, nil
	}

	// Instalar Chromium
	m.logger.Info("CHROMIUM", "� Installing Chromium automatically...")
	m.logger.Info("CHROMIUM", "⏳ This may take a few minutes (~150MB download)...")

	path, err := installer.InstallChromium(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to install chromium: %w", err)
	}

	return path, nil
}

// getCommonChromiumPaths retorna rutas comunes donde buscar Chromium
func (m *ChromiumManager) getCommonChromiumPaths() []string {
	switch runtime.GOOS {
	case "darwin": // macOS
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
			filepath.Join(os.Getenv("HOME"), "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
			filepath.Join(os.Getenv("HOME"), "Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge"),
		}
	case "linux":
		return []string{
			"/usr/bin/google-chrome",
			"/usr/bin/microsoft-edge",
			"/usr/bin/microsoft-edge-stable",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
			"/usr/bin/google-chrome-stable",
		}
	case "windows":
		programFiles := os.Getenv("ProgramFiles")
		programFilesX86 := os.Getenv("ProgramFiles(x86)")
		localAppData := os.Getenv("LOCALAPPDATA")
		return []string{
			filepath.Join(programFiles, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFiles, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(programFilesX86, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(programFilesX86, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(localAppData, "Google", "Chrome", "Application", "chrome.exe"),
			filepath.Join(programFiles, "Chromium", "Application", "chrome.exe"),
		}
	default:
		return []string{}
	}
}

// getLocalChromiumPath retorna la ruta donde instalar Chromium localmente
func (m *ChromiumManager) getLocalChromiumPath() string {
	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, "."+m.brand.Name, "chromium")

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(baseDir, "Chromium.app", "Contents", "MacOS", "Chromium")
	case "linux":
		return filepath.Join(baseDir, "chrome")
	case "windows":
		return filepath.Join(baseDir, "chrome.exe")
	default:
		return filepath.Join(baseDir, "chrome")
	}
}

// isValidChromium verifica si el path apunta a un ejecutable válido de Chromium
func (m *ChromiumManager) isValidChromium(path string) bool {
	if path == "" {
		return false
	}

	// Verificar que el archivo existe
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// En macOS, puede ser un bundle (.app), en otros OS debe ser ejecutable
	if runtime.GOOS == "darwin" {
		// En macOS aceptar tanto el bundle como el ejecutable
		return info.Mode().IsRegular() || info.IsDir()
	}

	// En Linux/Windows debe ser un archivo regular ejecutable
	return info.Mode().IsRegular() && (info.Mode().Perm()&0111 != 0 || runtime.GOOS == "windows")
}

// chromiumNeedsNoSandbox decide si hay que desactivar el sandbox de Chrome y
// por qué, para que el llamador pueda loguear un rastro auditable en ambos
// casos (ver docs/SECURITY_AUDIT_2026-07.md, AL-1).
func chromiumNeedsNoSandbox() (bool, string) {
	if os.Getenv("CHROMIUM_NO_SANDBOX") == "1" {
		return true, "CHROMIUM_NO_SANDBOX=1"
	}
	// Geteuid() no existe conceptualmente en Windows (siempre retorna -1);
	// el sandbox de Chrome ahí no depende de privilegios de root.
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		return true, "proceso corriendo como root"
	}
	return false, ""
}

// CreateContext crea un contexto de chromedp con el path detectado
func (m *ChromiumManager) CreateContext(parentCtx context.Context, chromiumPath string) (context.Context, context.CancelFunc, error) {
	// Check environment variable for debug mode
	debugMode := os.Getenv("CHROMIUM_DEBUG") == "1"

	// Opciones de chromedp. IMPORTANTE (ver docs/SECURITY_AUDIT_2026-07.md,
	// AL-1): NO forzar Flag("no-sandbox", true) incondicionalmente. Un
	// renderer de Chrome sin sandbox que procesa contenido no confiable más
	// scripts de CDN degrada un bug de corrupción de memoria a ejecución de
	// código como el usuario de build/CI.
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromiumPath),
		chromedp.Flag("headless", !debugMode), // Not headless if debug mode
		chromedp.Flag("disable-gpu", !debugMode),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-background-networking", true),
	)

	// El sandbox se desactiva en dos casos, ambos logueados para dejar
	// rastro auditable de cuándo se pierde esta protección:
	//   1. Root detectado (típico en contenedores Linux, donde el sandbox de
	//      Chrome requiere privilegios que un contenedor sin CAP_SYS_ADMIN
	//      no tiene). Lo detectamos aquí explícitamente (en vez de confiar
	//      en el fallback silencioso de chromedp's propio ExecAllocator —
	//      allocate.go:159, `os.Getuid() == 0` — que hace exactamente esto
	//      pero sin loguear nada) para que un build corriendo como root dentro
	//      de un contenedor deje un warning visible, no un silencio.
	//   2. Override explícito del operador vía CHROMIUM_NO_SANDBOX=1, para
	//      entornos no-root que igual no pueden usar el sandbox (p. ej. CI
	//      restringido sin user namespaces).
	if needsNoSandbox, reason := chromiumNeedsNoSandbox(); needsNoSandbox {
		opts = append(opts, chromedp.Flag("no-sandbox", true))
		m.logger.Warn("CHROMIUM", "⚠️  Sandbox de Chrome deshabilitado (%s) — ver docs/SECURITY_AUDIT_2026-07.md, AL-1", reason)
	}

	if debugMode {
		m.logger.Info("CHROMIUM", "🔍 DEBUG MODE: Browser window will be visible")
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(parentCtx, opts...)

	// Crear contexto del browser
	ctx, cancel := chromedp.NewContext(allocCtx)

	// Combinar los cancels
	combinedCancel := func() {
		cancel()
		allocCancel()
	}

	return ctx, combinedCancel, nil
}

// GetVersion obtiene la versión de Chromium instalado
func (m *ChromiumManager) GetVersion(chromiumPath string) string {
	cmd := exec.Command(chromiumPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}
