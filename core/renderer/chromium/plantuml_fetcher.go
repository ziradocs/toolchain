// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go.ziradocs.com/core/renderer"
	"go.ziradocs.com/core/util"
)

// plantumlMaxResponseBytes acota la respuesta del servidor PlantUML (SVG/PNG)
// para que un servidor hostil no agote memoria/disco con una respuesta de
// tamaño arbitrario (el diagrama más grande visto en la práctica es de unos
// pocos MB; 20 MB deja margen generoso sin permitir una respuesta ilimitada).
// `var`, no `const`, únicamente para poder acotarlo en tests sin generar
// decenas de MB reales; el código de producción nunca lo reasigna.
var plantumlMaxResponseBytes int64 = 20 * 1024 * 1024

// plantumlMaxRedirects acota cuántos redirects se siguen tras la petición
// inicial.
const plantumlMaxRedirects = 5

// PlantUMLFetcher maneja la descarga de imágenes PlantUML durante build time.
//
// Usa DOS clientes HTTP distintos en vez de uno con CheckRedirect, a
// propósito: la petición INICIAL va al servidor de `--plantuml-server` (un
// flag del operador, confiable — mismo trust level que `--theme`, ver el
// patrón "trusted bool" de loader.go), así que un servidor PlantUML interno/
// self-hosted en una red privada sigue funcionando sin restricción de IP.
// Cada REDIRECT, en cambio, lo decide el servidor en tiempo de respuesta —
// no el operador — así que si ese servidor está comprometido o es un mirror
// hostil, podría intentar pivotar el build hacia una IP interna (ver
// docs/SECURITY_AUDIT_2026-07.md, ME-3); esos hops usan restrictedClient,
// cuyo net.Dialer.Control valida la IP a la que REALMENTE se va a conectar
// en el momento del Dial — no una resolución de DNS por separado hecha
// antes (un CheckRedirect que llama net.LookupIP y luego deja que el
// Transport re-resuelva el host por su cuenta al conectar deja una ventana
// de DNS-rebinding: el DNS del atacante puede responder distinto en la
// consulta de validación que en la consulta real del Dial un instante
// después). Control se ejecuta exactamente en el momento de conectar, así
// que no hay nada que re-resolver de forma independiente.
type PlantUMLFetcher struct {
	server           string
	format           string
	trustedClient    *http.Client
	restrictedClient *http.Client
	outputDir        string // Directorio base de output (ej: ./output)
}

// NewPlantUMLFetcher crea un nuevo fetcher
func NewPlantUMLFetcher(server, format, outputDir string) *PlantUMLFetcher {
	if server == "" {
		server = "https://www.plantuml.com/plantuml"
	}
	if format == "" {
		format = "svg"
	}

	const timeout = 30 * time.Second
	return &PlantUMLFetcher{
		server:           strings.TrimSuffix(server, "/"),
		format:           format,
		trustedClient:    newNoRedirectClient(timeout, nil),
		restrictedClient: newNoRedirectClient(timeout, ssrfSafeDialControl),
		outputDir:        outputDir,
	}
}

// isBlockedFetchTarget reporta si ip no debe ser alcanzado al seguir un
// redirect de build-time: loopback, rangos privados RFC1918, link-local
// (incluye el endpoint de metadata de nube 169.254.169.254), multicast o no
// especificada. Una IP que no parsea se bloquea por defecto (fail-closed).
func isBlockedFetchTarget(ipStr string) bool {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return true
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}

// ssrfSafeDialControl se pasa como net.Dialer.Control: Go ya resolvió el
// host a una IP concreta antes de invocar este callback y está a punto de
// conectar exactamente a esa dirección — es la única validación que no deja
// ventana de TOCTOU/DNS-rebinding entre "resolver" y "conectar".
func ssrfSafeDialControl(network, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("invalid address %q: %w", address, err)
	}
	if isBlockedFetchTarget(host) {
		return fmt.Errorf("connection to %s blocked: private/loopback/link-local address not allowed", host)
	}
	return nil
}

// newNoRedirectClient construye un *http.Client que NUNCA sigue redirects
// automáticamente (fetchWithControlledRedirects decide manualmente, hop por
// hop, qué cliente usar para cada uno). dialControl, si no es nil, se
// instala como net.Dialer.Control.
func newNoRedirectClient(timeout time.Duration, dialControl func(network, address string, c syscall.RawConn) error) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, Control: dialControl}
	transport := &http.Transport{
		DialContext: dialer.DialContext,
		// Preserva el soporte de HTTP_PROXY/HTTPS_PROXY que un *http.Transport
		// vacío (a diferencia de http.DefaultTransport, usado antes de este
		// cambio) no trae por defecto.
		Proxy: http.ProxyFromEnvironment,
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// fetchWithControlledRedirects hace un GET a rawURL siguiendo redirects
// manualmente: la petición INICIAL usa f.trustedClient (sin restricción de
// IP); cada redirect posterior usa f.restrictedClient (valida la IP real de
// conexión vía Control). El caller es responsable de cerrar el
// resp.Body retornado.
func (f *PlantUMLFetcher) fetchWithControlledRedirects(ctx context.Context, rawURL string) (*http.Response, error) {
	client := f.trustedClient
	currentURL := rawURL

	for hop := 0; ; hop++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, currentURL, nil)
		if err != nil {
			return nil, fmt.Errorf("invalid URL %q: %w", currentURL, err)
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode < 300 || resp.StatusCode >= 400 {
			return resp, nil
		}
		location := resp.Header.Get("Location")
		_ = resp.Body.Close()
		if location == "" {
			return nil, fmt.Errorf("redirect response (status %d) had no Location header", resp.StatusCode)
		}

		if hop >= plantumlMaxRedirects {
			return nil, fmt.Errorf("stopped after %d redirects", plantumlMaxRedirects)
		}

		redirectURL, err := req.URL.Parse(location)
		if err != nil {
			return nil, fmt.Errorf("invalid redirect location %q: %w", location, err)
		}
		if redirectURL.Scheme != "https" {
			return nil, fmt.Errorf("redirect to non-https scheme %q blocked", redirectURL.Scheme)
		}

		currentURL = redirectURL.String()
		client = f.restrictedClient // todo hop DESPUÉS del primero usa el cliente restringido
	}
}

// FetchDiagramToAssets descarga un diagrama y lo guarda en assets/diagrams/
// Retorna la ruta relativa (assets/diagrams/xxx.svg) y error si falla
func (f *PlantUMLFetcher) FetchDiagramToAssets(ctx context.Context, content string) (string, error) {
	// Generar hash del contenido para nombre único
	hash := f.generateHash(content)
	filename := fmt.Sprintf("plantuml_%s.%s", hash, f.format)

	// Crear directorio assets/diagrams si no existe
	assetsDir := filepath.Join(f.outputDir, "assets", "diagrams")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create assets directory: %w", err)
	}

	// Path completo del archivo
	fullPath := filepath.Join(assetsDir, filename)

	// Si ya existe, no descargarlo de nuevo (cache)
	if _, err := os.Stat(fullPath); err == nil {
		return "assets/diagrams/" + filename, nil
	}

	// Generar URL del diagrama
	diagramURL := renderer.GeneratePlantUMLURL(content, f.server, f.format)

	// Descargar imagen
	resp, err := f.fetchWithControlledRedirects(ctx, diagramURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch diagram: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// util.ConsumeResponseToTempFile (issue #75 -- antes era una
	// implementacion independiente y duplicada de la misma logica que usa
	// chromium_installer.go) valida el status Y descarga acotada a un
	// temporal atomico; en error, limpia su propio temporal -- tmpPath solo
	// es valido cuando err == nil. El wrap usa un mensaje neutral ("failed
	// to fetch diagram", no "failed to write file"): el error puede venir
	// de un status remoto malo (servidor PlantUML caido/mal configurado),
	// no solo de un problema local de escritura -- un mensaje que asuma lo
	// segundo confunde el diagnostico cuando la causa real es lo primero
	// (hallazgo de code-review de PR #148).
	tmpPath, _, err := util.ConsumeResponseToTempFile(resp, assetsDir, "plantuml-*.tmp", plantumlMaxResponseBytes)
	if err != nil {
		return "", fmt.Errorf("failed to fetch diagram: %w", err)
	}
	defer func() { _ = os.Remove(tmpPath) }() // no-op tras el rename exitoso

	if err := os.Rename(tmpPath, fullPath); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	// Retornar path relativo
	return "assets/diagrams/" + filename, nil
}

// FetchDiagramInline descarga un diagrama y retorna el contenido SVG como string
// Solo funciona con format="svg"
func (f *PlantUMLFetcher) FetchDiagramInline(ctx context.Context, content string) (string, error) {
	if f.format != "svg" {
		return "", fmt.Errorf("inline mode only supports SVG format")
	}

	// Generar URL del diagrama
	diagramURL := renderer.GeneratePlantUMLURL(content, f.server, f.format)

	// Descargar SVG
	resp, err := f.fetchWithControlledRedirects(ctx, diagramURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch diagram: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// util.ConsumeResponseWithLimit (issue #75) valida el status Y acota la
	// lectura a memoria -- un servidor hostil no debe poder agotar memoria
	// con una respuesta de tamano arbitrario. Mensaje neutral ("failed to
	// fetch diagram", no "failed to read SVG"): el error puede venir de un
	// status remoto malo, no solo de un problema local de lectura (mismo
	// razonamiento que en FetchDiagramToAssets).
	svgBytes, err := util.ConsumeResponseWithLimit(resp, plantumlMaxResponseBytes)
	if err != nil {
		return "", fmt.Errorf("failed to fetch diagram: %w", err)
	}

	return string(svgBytes), nil
}

// generateHash genera un hash SHA256 corto del contenido
func (f *PlantUMLFetcher) generateHash(content string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	hash := hasher.Sum(nil)
	// Usar solo los primeros 12 caracteres del hash
	return hex.EncodeToString(hash)[:12]
}
