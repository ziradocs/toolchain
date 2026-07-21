// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"go.ziradocs.com/core/util"
)

const (
	// chromiumVersion es la versión de Chrome for Testing que este binario
	// descarga e instala. Ver docs/SECURITY_AUDIT_2026-07.md, ME-1: mantener
	// esto en una versión estable reciente (no fijarla por años) reduce la
	// ventana de exposición a CVEs conocidos del renderer.
	//
	// Para actualizar (junto con chromiumSHA256 más abajo):
	//   1. Consultar la versión Stable actual:
	//      curl -s https://googlechromelabs.github.io/chrome-for-testing/last-known-good-versions.json
	//   2. Para cada plataforma soportada (mac-arm64, mac-x64, linux64, win64),
	//      descargar el zip y calcular su hash:
	//      curl -sL <url-de-la-plataforma> | shasum -a 256
	//   3. Actualizar chromiumVersion y las 4 entradas de chromiumSHA256 en el
	//      mismo commit — nunca solo una de las dos (ver AL-2: el hash debe
	//      corresponder exactamente a la versión que se descarga).
	chromiumVersion = "150.0.7871.49"
)

// chromiumDownloadTimeout y chromiumMaxDownloadBytes son `var` (no `const`)
// únicamente para poder acotarlos en tests sin esperar minutos/cientos de MB
// reales; el código de producción nunca los reasigna.
var (
	// chromiumDownloadTimeout cubre la conexión completa (no solo el primer
	// byte); un timeout ausente permite que una descarga cuelgue el proceso
	// indefinidamente (ver docs/SECURITY_AUDIT_2026-07.md, AL-2).
	chromiumDownloadTimeout = 5 * time.Minute

	// chromiumMaxDownloadBytes acota el tamaño aceptado (~130-200MB reales
	// por plataforma); sin este cap, una respuesta manipulada o un servidor
	// comprometido podría intentar agotar disco/memoria indefinidamente.
	chromiumMaxDownloadBytes int64 = 400 * 1024 * 1024 // 400 MB

	// chromiumMaxUncompressedEntryBytes / chromiumMaxUncompressedTotalBytes
	// acotan la extracción del zip contra un zip-bomb: una entrada pequeña
	// en disco que declara un contenido descomprimido arbitrariamente
	// grande. Los límites son generosos frente al tamaño real de la
	// instalación (~300-500MB descomprimidos por plataforma) pero acotados,
	// para que una entrada maliciosa no agote disco indefinidamente (ver
	// docs/SECURITY_AUDIT_2026-07.md, BA-8).
	chromiumMaxUncompressedEntryBytes int64 = 600 * 1024 * 1024  // 600 MB
	chromiumMaxUncompressedTotalBytes int64 = 1500 * 1024 * 1024 // 1.5 GB
)

// chromiumSHA256 son los hashes SHA-256 esperados para chromiumVersion, uno
// por plataforma (clave: runtime.GOOS+"/"+runtime.GOARCH), calculados
// manualmente al fijar esta versión — Chrome for Testing no publica un
// manifiesto de checksums verificable, así que el hash "de confianza" es el
// que este repo fija y revisa en cada bump. Sin esta verificación, un proxy
// TLS-intercepting corporativo, un mirror comprometido o cache-poisoning
// podían servir un binario distinto y ejecutarlo como navegador de build sin
// ninguna comprobación (ver docs/SECURITY_AUDIT_2026-07.md, AL-2).
var chromiumSHA256 = map[string]string{
	"darwin/arm64":  "7bfe03fd3554cbf128a4517b014a8f74363e0bc6360a225043ed0ab6e1ea5b72",
	"darwin/amd64":  "7663b761c7c9a07fe594697a3fdab4875d9e4681b28b7fcb4cf4d79f5c18d684",
	"linux/amd64":   "51b1137390031ea031b06fae081615860d03d85efcf2853f0c430cee0161a781",
	"windows/amd64": "9323ea140e1da78a1ba4814f7174ce3f3cd6e850ff0e36a2c3bb3f861682cee0",
}

// ChromiumInstaller gestiona la descarga e instalación de Chromium
type ChromiumInstaller struct {
	logger ChromiumLogger
	brand  ChromiumBrand
}

// NewChromiumInstaller crea una nueva instancia del instalador con el
// branding por defecto de doclang (issue #92).
func NewChromiumInstaller(logger ChromiumLogger) *ChromiumInstaller {
	return newChromiumInstaller(logger, DefaultChromiumBrand)
}

// newChromiumInstaller es la construcción real; brand se inyecta por
// instancia en vez de leerse de estado global (issue #92). Sin exportar:
// solo ChromiumManager.downloadChromium necesita pasar su propio brand.
func newChromiumInstaller(logger ChromiumLogger, brand ChromiumBrand) *ChromiumInstaller {
	return &ChromiumInstaller{
		logger: logger,
		brand:  brand,
	}
}

// InstallChromium descarga e instala Chromium en el sistema. ctx acota/
// cancela la descarga (issue #134/G1d).
func (ci *ChromiumInstaller) InstallChromium(ctx context.Context) (string, error) {
	ci.logger.Info("CHROMIUM", "Installing Chromium automatically...")

	// 1. Determinar URL de descarga según OS
	downloadURL, err := ci.getDownloadURL()
	if err != nil {
		return "", fmt.Errorf("failed to determine download URL: %w", err)
	}

	ci.logger.Info("CHROMIUM", "Download URL: %s", downloadURL)

	// 2. Crear directorio de instalación
	installDir, err := ci.getInstallDir()
	if err != nil {
		return "", fmt.Errorf("failed to create install directory: %w", err)
	}

	ci.logger.Info("CHROMIUM", "Install directory: %s", installDir)

	// 3. Descargar archivo
	zipPath := filepath.Join(installDir, "chromium.zip")
	ci.logger.Info("CHROMIUM", "Downloading Chromium (this may take a few minutes)...")

	if err := ci.downloadFile(ctx, downloadURL, zipPath); err != nil {
		return "", fmt.Errorf("failed to download Chromium: %w", err)
	}

	ci.logger.Info("CHROMIUM", "✅ Download complete")

	// 4. Descomprimir
	ci.logger.Info("CHROMIUM", "Extracting Chromium...")
	extractDir := filepath.Join(installDir, "chromium")

	if err := ci.extractZip(zipPath, extractDir); err != nil {
		return "", fmt.Errorf("failed to extract Chromium: %w", err)
	}

	// 5. Eliminar archivo zip
	_ = os.Remove(zipPath) // Ignorar error si no se puede eliminar

	ci.logger.Info("CHROMIUM", "✅ Extraction complete")

	// 6. Encontrar binario ejecutable
	execPath, err := ci.findExecutable(extractDir)
	if err != nil {
		return "", fmt.Errorf("failed to find Chromium executable: %w", err)
	}

	// 7. Configurar permisos (Unix/Mac)
	if runtime.GOOS != "windows" {
		if err := os.Chmod(execPath, 0755); err != nil {
			return "", fmt.Errorf("failed to set executable permissions: %w", err)
		}
	}

	ci.logger.Info("CHROMIUM", "✅ Chromium installed successfully at: %s", execPath)

	return execPath, nil
}

// chromiumPlatformKey identifica la plataforma actual para buscar su hash
// esperado en chromiumSHA256.
func chromiumPlatformKey() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

// getDownloadURL retorna la URL de descarga según el sistema operativo
func (ci *ChromiumInstaller) getDownloadURL() (string, error) {
	// Usamos Chromium for Testing builds (estables y confiables)
	// https://googlechromelabs.github.io/chrome-for-testing/

	if _, ok := chromiumSHA256[chromiumPlatformKey()]; !ok {
		return "", fmt.Errorf("unsupported platform for auto-install: %s (no pinned checksum)", chromiumPlatformKey())
	}

	baseURL := "https://storage.googleapis.com/chrome-for-testing-public"

	switch runtime.GOOS {
	case "darwin":
		// macOS (arm64 o x64)
		if runtime.GOARCH == "arm64" {
			return fmt.Sprintf("%s/%s/mac-arm64/chrome-mac-arm64.zip", baseURL, chromiumVersion), nil
		}
		return fmt.Sprintf("%s/%s/mac-x64/chrome-mac-x64.zip", baseURL, chromiumVersion), nil

	case "linux":
		// Linux x64
		return fmt.Sprintf("%s/%s/linux64/chrome-linux64.zip", baseURL, chromiumVersion), nil

	case "windows":
		// Windows x64
		return fmt.Sprintf("%s/%s/win64/chrome-win64.zip", baseURL, chromiumVersion), nil

	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

// getInstallDir retorna el directorio donde se instalará Chromium
func (ci *ChromiumInstaller) getInstallDir() (string, error) {
	// Usar directorio de cache del sistema
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	var cacheDir string
	switch runtime.GOOS {
	case "darwin":
		cacheDir = filepath.Join(homeDir, "Library", "Caches", ci.brand.Name)
	case "linux":
		cacheDir = filepath.Join(homeDir, ".cache", ci.brand.Name)
	case "windows":
		cacheDir = filepath.Join(homeDir, "AppData", "Local", ci.brand.Name, "Cache")
	default:
		cacheDir = filepath.Join(homeDir, "."+ci.brand.Name, "cache")
	}

	// Crear directorio si no existe
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", err
	}

	return cacheDir, nil
}

// downloadProgressLoggerIntervalMB controla cada cuantos MB descargados se
// loguea una linea de progreso.
const downloadProgressLoggerIntervalMB = 20

// downloadProgressLogger es un io.Writer que solo cuenta bytes para loguear
// el progreso de la descarga cada downloadProgressLoggerIntervalMB MB; se
// pasa como extraWriter a util.DownloadToTempFile junto al hasher SHA-256.
// A diferencia de la version anterior (pre issue #75), ya no muestra un
// porcentaje sobre el tamano total -- util.DownloadToTempFile no expone
// resp.ContentLength al caller antes de empezar a copiar, asi que el
// progreso ahora se reporta en MB descargados acumulados; UX ligeramente
// distinta, mismo proposito (feedback durante una descarga lenta).
type downloadProgressLogger struct {
	logger       ChromiumLogger
	downloaded   int64
	lastLoggedMB int64
}

func (p *downloadProgressLogger) Write(b []byte) (int, error) {
	n := len(b)
	p.downloaded += int64(n)
	downloadedMB := p.downloaded / (1024 * 1024)
	if downloadedMB >= p.lastLoggedMB+downloadProgressLoggerIntervalMB {
		p.logger.Info("CHROMIUM", "Downloaded %d MB so far...", downloadedMB)
		p.lastLoggedMB = downloadedMB
	}
	return n, nil
}

// downloadFile descarga un archivo desde una URL, verificando su SHA-256
// contra chromiumSHA256 antes de exponerlo en destPath (ver
// docs/SECURITY_AUDIT_2026-07.md, AL-2). El mecanismo de descarga acotada +
// temporal atómico vive en util.ConsumeResponseToTempFile (issue #75: antes
// era una implementación independiente y duplicada de la misma lógica que
// usa renderer/plantuml_fetcher.go) — acá solo queda lo específico de
// Chromium: cómo se obtiene la respuesta (un client.Get simple, sin la
// lógica de redirects SSRF-safe que sí necesita PlantUML) y el hash
// esperado por plataforma; el rename final solo ocurre si el hash coincide,
// así que nunca queda un binario corrupto o manipulado en el path que luego
// se marca ejecutable.
func (ci *ChromiumInstaller) downloadFile(ctx context.Context, url, destPath string) error {
	expectedHash, ok := chromiumSHA256[chromiumPlatformKey()]
	if !ok {
		return fmt.Errorf("no pinned SHA-256 for platform %s", chromiumPlatformKey())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("invalid URL %q: %w", url, err)
	}

	client := &http.Client{Timeout: chromiumDownloadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	hasher := sha256.New()
	progress := &downloadProgressLogger{logger: ci.logger}

	// tmpPath solo es válido (no vacío) cuando err == nil — en error,
	// util.ConsumeResponseToTempFile ya limpió su propio temporal.
	tmpPath, written, err := util.ConsumeResponseToTempFile(resp, filepath.Dir(destPath), "chromium-download-*.tmp", chromiumMaxDownloadBytes, hasher, progress)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmpPath) }() // no-op tras el rename exitoso

	ci.logger.Info("CHROMIUM", "Downloaded %.2f MB", float64(written)/(1024*1024))

	actualHash := hex.EncodeToString(hasher.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("SHA-256 mismatch for %s: expected %s, got %s -- aborting (possible corrupted or tampered download)", url, expectedHash, actualHash)
	}

	return os.Rename(tmpPath, destPath)
}

// symlinkTargetMaxBytes acota la lectura del "contenido" de una entrada
// symlink (que en un zip es literalmente el texto del target, no un archivo
// real) — ninguna ruta de filesystem legítima se acerca a este tamaño; sirve
// solo para no confiar en un tamaño declarado arbitrario al leerla.
const symlinkTargetMaxBytes = 4096

// extractZip descomprime un archivo zip. destDir se limpia por completo
// antes de extraer (una instalación previa parcial/fallida no debe
// mezclarse con la nueva) y, si la extracción falla a mitad de camino, se
// vuelve a limpiar antes de retornar el error — un install fallido no debe
// dejar un árbol incompleto que IsChromiumInstalled (que solo mira el
// binario ejecutable, no el árbol completo) confunda con uno completo.
func (ci *ChromiumInstaller) extractZip(zipPath, destDir string) (err error) {
	// Abrir archivo zip
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	if err := os.RemoveAll(destDir); err != nil {
		return err
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(destDir)
		}
	}()

	cleanDestDir := filepath.Clean(destDir)

	// Extraer cada archivo
	var totalWritten int64
	for _, file := range reader.File {
		// Construir path destino
		path := filepath.Join(destDir, file.Name)

		// Verificar que el path esté dentro del directorio destino (seguridad)
		if !strings.HasPrefix(path, cleanDestDir+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", file.Name)
		}

		// El chequeo de arriba es puramente LÉXICO sobre el nombre de la
		// entrada — no detecta que una entrada ANTERIOR de este mismo zip ya
		// haya colocado un symlink en algún ancestro de path. Si lo hizo, el
		// kernel resuelve ese symlink al crear/escribir esta entrada, así
		// que el destino REAL puede terminar fuera de destDir aunque el
		// nombre de la entrada "luzca" confinado — y, para una entrada
		// symlink en particular, su propio chequeo de confinamiento en
		// extractSymlink razona sobre el padre LÉXICO (falso), no el padre
		// FÍSICO real, permitiendo construir un target que parece confinado
		// contra el padre falso pero escapa destDir una vez resuelto contra
		// el padre real. Rechazar cualquier entrada cuyo padre ya atraviese
		// un symlink creado por esta misma extracción cierra esa vía.
		if hasSymlink, err := pathHasSymlinkAncestor(cleanDestDir, path); err != nil {
			return err
		} else if hasSymlink {
			return fmt.Errorf("illegal file path: %s (an ancestor directory is a symlink)", file.Name)
		}

		if file.Mode()&os.ModeSymlink != 0 {
			if err := ci.extractSymlink(file, path, cleanDestDir); err != nil {
				return err
			}
			continue
		}

		if file.FileInfo().IsDir() {
			// Crear directorio
			_ = os.MkdirAll(path, file.Mode())
			continue
		}

		// Crear directorios padres si no existen
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}

		// El presupuesto de ESTA entrada es el menor entre el cap por-entrada
		// y lo que queda del cap total — así una entrada individual nunca
		// puede hacer que el total escriba de más del cap documentado (antes,
		// el cap total solo se chequeaba DESPUÉS de que la entrada completa
		// ya se hubiera escrito, permitiendo un overshoot de hasta un
		// cap-por-entrada completo).
		remaining := chromiumMaxUncompressedTotalBytes - totalWritten
		if remaining <= 0 {
			return fmt.Errorf("zip extraction exceeds max total uncompressed size (%d MB)", chromiumMaxUncompressedTotalBytes/(1024*1024))
		}
		entryCap := chromiumMaxUncompressedEntryBytes
		cappedByTotal := false
		if remaining < entryCap {
			entryCap = remaining
			cappedByTotal = true
		}

		// Extraer archivo
		written, extractErr := ci.extractFile(file, path, entryCap)
		totalWritten += written
		if extractErr != nil {
			// Si el presupuesto de esta llamada vino del total restante (no
			// del cap por-entrada), el mensaje debe decir "total", no
			// "por-entrada" — de lo contrario un caso como "dos entries de
			// 10 bytes con un total-cap de 15" reportaría erróneamente un
			// límite por-entrada casi cero.
			if cappedByTotal {
				return fmt.Errorf("zip extraction exceeds max total uncompressed size (%d MB): %w", chromiumMaxUncompressedTotalBytes/(1024*1024), extractErr)
			}
			return extractErr
		}
	}

	return nil
}

// pathHasSymlinkAncestor reporta si algún directorio entre cleanDestDir y el
// padre de path ya es, en el filesystem real, un symlink — sin importar si
// ese symlink lo creó una entrada anterior de esta misma extracción o algo
// preexistente. destDir mismo nunca se chequea (ya se limpia y recrea al
// inicio de extractZip). No requiere que path exista todavía: camina hacia
// arriba desde su padre, Lstat-eando cada componente que sí exista.
func pathHasSymlinkAncestor(cleanDestDir, path string) (bool, error) {
	dir := filepath.Dir(path)
	destDirWithSep := cleanDestDir + string(os.PathSeparator)
	for len(dir) > len(cleanDestDir) && strings.HasPrefix(dir, destDirWithSep) {
		info, err := os.Lstat(dir)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return true, nil
		}
		if err != nil && !os.IsNotExist(err) {
			return false, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return false, nil
}

// extractSymlink crea una entrada symlink del zip, confinando su TARGET a
// destDir. El nombre de la entrada en sí ya se validó en extractZip, pero
// eso solo cubre DÓNDE vive el symlink — no a dónde APUNTA; un target
// absoluto o con ".." puede escapar destDir sin que el chequeo de nombre lo
// note (ver docs/SECURITY_AUDIT_2026-07.md, BA-8). Los symlinks legítimos de
// un bundle de Chrome for Testing en macOS (framework bundles, p. ej.
// Versions/Current -> A) son siempre relativos y confinados al propio
// bundle, así que rechazarlos de plano (en vez de validar el target) rompía
// --install-chromium en Mac.
func (ci *ChromiumInstaller) extractSymlink(file *zip.File, path, cleanDestDir string) error {
	srcFile, err := file.Open()
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	targetBytes, err := io.ReadAll(io.LimitReader(srcFile, symlinkTargetMaxBytes+1))
	if err != nil {
		return fmt.Errorf("failed to read symlink target for %s: %w", file.Name, err)
	}
	if len(targetBytes) > symlinkTargetMaxBytes {
		return fmt.Errorf("symlink target for %s exceeds max length", file.Name)
	}
	target := string(targetBytes)

	if filepath.IsAbs(target) {
		return fmt.Errorf("symlink %s has an absolute target %q, which is not allowed", file.Name, target)
	}

	resolvedTarget := filepath.Join(filepath.Dir(path), target)
	if resolvedTarget != cleanDestDir && !strings.HasPrefix(resolvedTarget, cleanDestDir+string(os.PathSeparator)) {
		return fmt.Errorf("symlink %s target %q escapes the destination directory", file.Name, target)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.Symlink(target, path); err != nil {
		return fmt.Errorf("failed to create symlink %s -> %s: %w", path, target, err)
	}
	return nil
}

// extractFile extrae un archivo individual del zip y retorna los bytes
// escritos (para que extractZip pueda acumular el total descomprimido).
// maxBytes es el presupuesto de ESTA entrada (ver comentario en extractZip
// sobre por qué no es simplemente chromiumMaxUncompressedEntryBytes).
func (ci *ChromiumInstaller) extractFile(file *zip.File, destPath string, maxBytes int64) (int64, error) {
	// Abrir archivo en zip
	srcFile, err := file.Open()
	if err != nil {
		return 0, err
	}
	defer func() { _ = srcFile.Close() }()

	// O_EXCL hace que la apertura falle atómicamente si destPath YA EXISTE
	// (de cualquier tipo, incluido un symlink) — a diferencia de un Lstat
	// previo seguido de Open, no deja ventana TOCTOU entre "verificar" y
	// "abrir". destDir se limpia por completo al inicio de extractZip, así
	// que ninguna entrada legítima debería encontrar un destPath preexistente
	// aquí; si lo encuentra, es una señal de zip malformado (nombres
	// duplicados) o de una carrera externa, y en ambos casos abortar es lo
	// correcto.
	destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, file.Mode())
	if err != nil {
		return 0, err
	}
	defer func() { _ = destFile.Close() }()

	// io.LimitReader(..., max+1) deja detectar el exceso sin agotar
	// srcFile contando manualmente por chunk (mismo patrón que downloadFile).
	written, err := io.Copy(destFile, io.LimitReader(srcFile, maxBytes+1))
	if err != nil {
		return written, err
	}
	if written > maxBytes {
		return written, fmt.Errorf("zip entry %s exceeds max uncompressed size (%d MB)", file.Name, maxBytes/(1024*1024))
	}
	return written, nil
}

// findExecutable encuentra el binario ejecutable de Chromium
func (ci *ChromiumInstaller) findExecutable(extractDir string) (string, error) {
	var execName string

	switch runtime.GOOS {
	case "darwin":
		// macOS: Chrome.app/Contents/MacOS/Chrome
		if runtime.GOARCH == "arm64" {
			execName = "chrome-mac-arm64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing"
		} else {
			execName = "chrome-mac-x64/Google Chrome for Testing.app/Contents/MacOS/Google Chrome for Testing"
		}

	case "linux":
		// Linux: chrome
		execName = "chrome-linux64/chrome"

	case "windows":
		// Windows: chrome.exe
		execName = "chrome-win64/chrome.exe"

	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	execPath := filepath.Join(extractDir, execName)

	// Verificar que el archivo existe
	if _, err := os.Stat(execPath); err != nil {
		return "", fmt.Errorf("executable not found at %s: %w", execPath, err)
	}

	return execPath, nil
}

// IsChromiumInstalled verifica si Chromium ya está instalado
func (ci *ChromiumInstaller) IsChromiumInstalled() (bool, string) {
	installDir, err := ci.getInstallDir()
	if err != nil {
		return false, ""
	}

	extractDir := filepath.Join(installDir, "chromium")
	execPath, err := ci.findExecutable(extractDir)
	if err != nil {
		return false, ""
	}

	// Verificar que el archivo existe y es ejecutable
	info, err := os.Stat(execPath)
	if err != nil {
		return false, ""
	}

	// En Unix, verificar permisos de ejecución
	if runtime.GOOS != "windows" {
		if info.Mode()&0111 == 0 {
			return false, ""
		}
	}

	return true, execPath
}
