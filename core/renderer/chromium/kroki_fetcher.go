// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

//go:build !js

package chromium

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// kroki_fetcher.go implementa renderer.MermaidFetcher y
// renderer.PlantUMLFetcher contra un servidor Kroki (https://kroki.io
// público, o uno on-prem) por HTTP puro — sin Chromium en la máquina que
// corre el build. Kroki sí usa Chromium/Puppeteer internamente para
// mermaid, pero del lado del servidor: eso es responsabilidad de quien
// opera el servidor Kroki, no de este binario.
//
// Mismo modelo de confianza que plantuml_fetcher.go, deliberadamente
// reusado: la petición inicial va a --kroki-server (un flag del operador,
// mismo trust level que --theme/--plantuml-server) sin restricción de IP,
// así que un Kroki on-prem en 10.x/192.168.x funciona sin configuración
// especial; cualquier REDIRECT que el servidor emita sí se valida contra
// rangos privados/loopback vía ssrfSafeDialControl (ver
// docs/SECURITY_AUDIT_2026-07.md, ME-3) y solo se sigue si es https — un
// Kroki servido en HTTP plano detrás de un balanceador que redirige a una
// IP interna no atraviesa ese segundo hop. Ninguna de las dos restricciones
// es nueva: son las mismas que ya rigen PlantUML.
//
// Usa el endpoint POST /{diagramType}/{outputFormat} de Kroki (la fuente
// del diagrama va en el body, texto plano) en vez del GET con URL
// codificada: el encoder de plantuml_encoder.go usa DEFLATE crudo + un
// alfabeto propio, distinto del zlib + base64-urlsafe estándar que Kroki
// espera en la URL — reescribirlo no compra nada frente a mandar el texto
// por POST, que además no está acotado por el límite de longitud de una
// URL.
const (
	krokiMaxResponseBytes int64 = 20 * 1024 * 1024
	krokiMaxRedirects           = 5
	krokiTimeout                = 30 * time.Second
)

// KrokiFetcher satisface renderer.MermaidFetcher (FetchAndSave/FetchInline)
// y renderer.PlantUMLFetcher (FetchDiagramToAssets/FetchDiagramInline) al
// mismo tiempo: son dos pares de nombres de método distintos sobre el mismo
// tipo, cada uno delegando en render(). diagramType selecciona el
// componente Kroki ("mermaid", "plantuml", ...) que arma cada instancia. No
// se declara explícitamente (Go structural typing, mismo patrón que el
// resto del paquete) — las aserciones de abajo lo verifican en compilación.
var (
	_ renderer.MermaidFetcher  = (*KrokiFetcher)(nil)
	_ renderer.PlantUMLFetcher = (*KrokiFetcher)(nil)
)

type KrokiFetcher struct {
	server           string
	diagramType      string
	trustedClient    *http.Client
	restrictedClient *http.Client
	outputDir        string

	// cache guarda el SVG ya obtenido por hash de contenido, compartido
	// entre las cuatro variantes de fetch (a diferencia de
	// BaseFetcher.FetchInline, que solo aparenta cachear y siempre vuelve a
	// renderizar — ver base_fetcher.go:140. Con Chromium ese re-render es
	// solo más lento; contra un servidor Kroki remoto son N llamadas HTTP
	// de más por el mismo diagrama).
	mu    sync.Mutex
	cache map[string][]byte
}

// NewKrokiFetcher crea un fetcher contra server (vacío = https://kroki.io
// público) para el componente diagramType ("mermaid" o "plantuml").
func NewKrokiFetcher(server, diagramType, outputDir string) *KrokiFetcher {
	if server == "" {
		server = "https://kroki.io"
	}
	return &KrokiFetcher{
		server:           strings.TrimSuffix(server, "/"),
		diagramType:      diagramType,
		trustedClient:    newNoRedirectClient(krokiTimeout, nil),
		restrictedClient: newNoRedirectClient(krokiTimeout, ssrfSafeDialControl),
		outputDir:        outputDir,
		cache:            make(map[string][]byte),
	}
}

// postWithControlledRedirects es el análogo POST de
// PlantUMLFetcher.fetchWithControlledRedirects: la petición inicial usa
// f.trustedClient (sin restricción de IP), cada redirect posterior usa
// f.restrictedClient. A diferencia del GET de PlantUML, reenvía el mismo
// body en cada hop (semántica 307/308) — Kroki normalmente no redirige en
// absoluto; esto solo cubre un despliegue detrás de un balanceador/ingress.
func (f *KrokiFetcher) postWithControlledRedirects(ctx context.Context, rawURL, body string) (*http.Response, error) {
	client := f.trustedClient
	currentURL := rawURL

	for hop := 0; ; hop++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, currentURL, strings.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("invalid URL %q: %w", currentURL, err)
		}
		req.Header.Set("Content-Type", "text/plain")

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

		if hop >= krokiMaxRedirects {
			return nil, fmt.Errorf("stopped after %d redirects", krokiMaxRedirects)
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

// cacheKey deriva la clave de caché (en memoria vía f.cache, y en disco vía
// el nombre de archivo que FetchAndSave/FetchDiagramToAssets escriben) para
// content. Incluye f.server además de diagramType+content: sin esto, dos
// builds del mismo documento en el mismo outputDir pero apuntando a
// --kroki-server distintos (p. ej. un servidor on-prem staging vs uno de
// producción) reusarían el SVG del servidor viejo — el os.Stat de
// FetchAndSave/FetchDiagramToAssets ve el mismo nombre de archivo y nunca
// vuelve a pedirlo al servidor nuevo. Único punto de derivación para que un
// quinto método futuro no repita la construcción a mano y se le olvide
// f.server.
func (f *KrokiFetcher) cacheKey(content string) string {
	return GenerateContentHash(f.server + "|" + f.diagramType + "|" + content)
}

// render obtiene el SVG de content, sirviendo desde f.cache cuando ya se
// pidió antes (por cualquiera de los cuatro métodos públicos).
func (f *KrokiFetcher) render(ctx context.Context, content string) ([]byte, error) {
	hash := f.cacheKey(content)

	f.mu.Lock()
	if cached, ok := f.cache[hash]; ok {
		f.mu.Unlock()
		return cached, nil
	}
	f.mu.Unlock()

	url := fmt.Sprintf("%s/%s/svg", f.server, f.diagramType)
	resp, err := f.postWithControlledRedirects(ctx, url, content)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch diagram from kroki: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := util.ConsumeResponseWithLimit(resp, krokiMaxResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch diagram from kroki: %w", err)
	}

	f.mu.Lock()
	f.cache[hash] = data
	f.mu.Unlock()
	return data, nil
}

// writeAtomic escribe data en filepath.Join(dir, filename) vía
// temp-file+rename (mismo patrón que PlantUMLFetcher.FetchDiagramToAssets,
// evita que un fallo a mitad de escritura deje un archivo parcial que un
// build posterior tomaría como cache válido).
func writeAtomic(dir, filename string, data []byte) error {
	tmp, err := os.CreateTemp(dir, "kroki-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }() // no-op tras el rename exitoso

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, filepath.Join(dir, filename))
}

// FetchAndSave satisface renderer.MermaidFetcher: renderiza code y lo
// guarda en <outputDir>/diagrams/, mismo directorio que
// chromium.MermaidFetcher (BaseFetcher con assetType "diagrams") para que
// un caller que alterne entre backend chromium y backend kroki no vea
// cambiar la convención de rutas.
func (f *KrokiFetcher) FetchAndSave(ctx context.Context, code string, outputDir string) (string, error) {
	hash := f.cacheKey(code)
	filename := fmt.Sprintf("kroki-%s_%s.svg", f.diagramType, hash)

	assetsDir := filepath.Join(outputDir, "diagrams")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create diagrams directory: %w", err)
	}
	relativePath := filepath.Join("diagrams", filename)

	if _, err := os.Stat(filepath.Join(assetsDir, filename)); err == nil {
		return relativePath, nil
	}

	data, err := f.render(ctx, code)
	if err != nil {
		return "", err
	}
	if err := writeAtomic(assetsDir, filename, data); err != nil {
		return "", fmt.Errorf("failed to save diagram: %w", err)
	}
	return relativePath, nil
}

// FetchInline satisface renderer.MermaidFetcher: renderiza code y retorna
// el SVG como string, sin escribir a disco.
func (f *KrokiFetcher) FetchInline(ctx context.Context, code string) (string, error) {
	data, err := f.render(ctx, code)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FetchDiagramToAssets satisface renderer.PlantUMLFetcher: mismo
// comportamiento que FetchAndSave, pero guardando bajo
// <outputDir>/assets/diagrams/ — la convención de PlantUMLFetcher, para
// sustituirlo sin que el HTML generado tenga que distinguir cuál de los dos
// lo produjo.
func (f *KrokiFetcher) FetchDiagramToAssets(ctx context.Context, content string) (string, error) {
	hash := f.cacheKey(content)
	filename := fmt.Sprintf("kroki-%s_%s.svg", f.diagramType, hash)

	assetsDir := filepath.Join(f.outputDir, "assets", "diagrams")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create assets directory: %w", err)
	}
	relativePath := "assets/diagrams/" + filename

	if _, err := os.Stat(filepath.Join(assetsDir, filename)); err == nil {
		return relativePath, nil
	}

	data, err := f.render(ctx, content)
	if err != nil {
		return "", err
	}
	if err := writeAtomic(assetsDir, filename, data); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}
	return relativePath, nil
}

// FetchDiagramInline satisface renderer.PlantUMLFetcher: renderiza content
// y retorna el SVG como string.
func (f *KrokiFetcher) FetchDiagramInline(ctx context.Context, content string) (string, error) {
	data, err := f.render(ctx, content)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
