// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/v2/util"
)

// validSource lleva frontmatter para ser genuinamente lint-clean: a
// diferencia de slidelang (donde la ausencia de "---" es un error FATAL de
// PARSEO, ver CLAUDE.md), DocumentFlexParser de doclang tolera un documento
// sin frontmatter a nivel de parseo — pero el LINTER compartido (misma
// regla FRONT003 que corre `slidelang build`) sí exige el bloque, así que
// omitirlo aquí haría Valid=false por una razón distinta a la que cada test
// quiere ejercitar.
const validSource = `---
title: "Test"
---

# Doc

Hello world.
`

// xrefSource trae una ecuación etiquetada y un \ref a esa misma etiqueta —
// prueba end-to-end de que parseSource es build-faithful (ver parse.go): la
// numeración y la resolución de \ref solo aparecen si la etapa de transform
// (transform.RunBuiltins + xref.Transform) corrió de verdad, no solo el
// parseo. Usa MATH (no TABLE/IMAGE): a diferencia de MathParser, que soporta
// `label:` en modo flex, TableParser/ImageParser solo leen `label:`/
// `caption:` en modo strict — inalcanzable en DocumentFlexParser, que
// siempre corre en modo flex. Un doc doclang real no puede hoy etiquetar una
// figura/tabla con la sintaxis flex/markdown que doclang realmente soporta
// (gap de #239-A verificado empíricamente al construir este test, ver la
// nota en el plan/PR — issue de seguimiento pendiente); las ecuaciones sí
// funcionan porque su parser no tiene esa restricción.
const xrefSource = "# Doc\n\n" +
	"See \\ref{eq:euler} below.\n\n" +
	"<<math>>\n" +
	"e^{i\\pi} + 1 = 0\n" +
	`label: "eq:euler"` + "\n" +
	"<<end>>\n"

// duplicateLabelSource dispara un error real de xref.Transform (label
// duplicado, xref/numbering.go) — a diferencia de slidelang, cuyo
// "parse error" de prueba es contenido sin frontmatter (fatal para
// slidelang, tolerado por doclang), este es el fallo de contenido genuino
// más simple de alcanzar en doclang: un error de Go propagado por
// parseSource, no un diagnóstico.
const duplicateLabelSource = "# Doc\n\n" +
	"<<math>>\na = b\n" + `label: "eq:dup"` + "\n<<end>>\n\n" +
	"<<math>>\nc = d\n" + `label: "eq:dup"` + "\n<<end>>\n"

// newTestSession arma un servidor MCP real (con todos sus tools) y un
// cliente conectados vía el transporte in-memory del SDK — E2E real, no un
// mock: ejercita el mismo ListTools/CallTool que vería un cliente MCP real
// contra `doclang mcp`.
func newTestSession(t *testing.T) (*sdkmcp.ClientSession, context.Context) {
	t.Helper()
	ctx := context.Background()

	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()

	server := NewServer(util.NewNoop())
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("server.Connect: %v", err)
	}

	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })

	return session, ctx
}

func callTool(t *testing.T, session *sdkmcp.ClientSession, ctx context.Context, name string, args map[string]any) *sdkmcp.CallToolResult {
	t.Helper()
	res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%q): %v", name, err)
	}
	return res
}

func resultText(t *testing.T, res *sdkmcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("expected exactly 1 content block, got %d", len(res.Content))
	}
	tc, ok := res.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	return tc.Text
}

func TestListTools(t *testing.T) {
	session, ctx := newTestSession(t)

	got, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	want := map[string]bool{"lint": false, "get_ast": false, "list_themes": false, "preview": false}
	for _, tool := range got.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("tool %q not registered", name)
		}
	}
}

func TestLintTool_Valid(t *testing.T) {
	session, ctx := newTestSession(t)

	res := callTool(t, session, ctx, "lint", map[string]any{"source": validSource})
	if res.IsError {
		t.Fatalf("expected success, got error result: %s", resultText(t, res))
	}

	var out lintOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.Valid {
		t.Errorf("expected Valid=true for well-formed content, diagnostics: %+v", out.Diagnostics)
	}
	if out.SectionCount != 1 {
		t.Errorf("expected SectionCount=1, got %d", out.SectionCount)
	}
}

func TestLintTool_TransformError(t *testing.T) {
	session, ctx := newTestSession(t)

	res := callTool(t, session, ctx, "lint", map[string]any{"source": duplicateLabelSource})
	if !res.IsError {
		t.Fatal("expected IsError=true: a duplicate label is a real content failure, not a warning")
	}

	var out lintOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Valid {
		t.Fatal("expected Valid=false for a duplicate xref label")
	}
	if !strings.Contains(out.Error, "duplicado") {
		t.Errorf("expected a duplicate-label error message, got: %q", out.Error)
	}
}

func TestGetASTTool_Valid(t *testing.T) {
	session, ctx := newTestSession(t)

	res := callTool(t, session, ctx, "get_ast", map[string]any{"source": validSource})
	if res.IsError {
		t.Fatalf("expected success, got error result: %s", resultText(t, res))
	}

	var doc map[string]any
	if err := json.Unmarshal([]byte(resultText(t, res)), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc["schemaVersion"] == nil {
		t.Error("expected schemaVersion field in AST JSON")
	}
	if doc["contentBlocks"] == nil {
		t.Error("expected contentBlocks field in AST JSON")
	}
}

// TestGetASTTool_BuildFaithfulNumbering es la prueba central de la decisión
// de diseño de este paquete (ver parse.go): get_ast debe llevar la misma
// numeración/resolución de \ref que produce `doclang build`, no solo el
// resultado crudo del parser. Si alguien reemplazara parseSource por una
// versión parse-only (como quedó la primera versión del MCP de slidelang,
// anterior a #239/#240), este test falla.
func TestGetASTTool_BuildFaithfulNumbering(t *testing.T) {
	session, ctx := newTestSession(t)

	res := callTool(t, session, ctx, "get_ast", map[string]any{"source": xrefSource})
	if res.IsError {
		t.Fatalf("expected success, got error result: %s", resultText(t, res))
	}

	text := resultText(t, res)
	if !strings.Contains(text, `"number":1`) {
		t.Errorf("expected the labeled equation to carry Number=1 (transform stage must have run), got: %s", text)
	}
	if !strings.Contains(text, "#eq-euler") {
		t.Errorf("expected \\ref{eq:euler} resolved to a #eq-euler anchor link, got: %s", text)
	}
	if strings.Contains(text, `\\ref{eq:euler}`) {
		t.Errorf("expected \\ref{eq:euler} to be resolved, not left verbatim, got: %s", text)
	}
}

func TestGetASTTool_TransformError(t *testing.T) {
	session, ctx := newTestSession(t)

	res := callTool(t, session, ctx, "get_ast", map[string]any{"source": duplicateLabelSource})
	if !res.IsError {
		t.Fatal("expected IsError=true: no valid AST can be built when the transform stage fails")
	}

	var out getASTError
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Error == "" {
		t.Error("expected a non-empty Error message for a transform-stage failure")
	}
}

func TestListThemesTool(t *testing.T) {
	session, ctx := newTestSession(t)

	res := callTool(t, session, ctx, "list_themes", map[string]any{})
	if res.IsError {
		t.Fatalf("expected success, got error result: %s", resultText(t, res))
	}

	var out listThemesOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Themes) == 0 {
		t.Fatal("expected at least the embedded themes (professional, academic, technical, page-view)")
	}
	names := make(map[string]bool, len(out.Themes))
	for _, th := range out.Themes {
		names[th.Name] = true
	}
	if !names["professional"] {
		t.Errorf("expected embedded 'professional' theme in list, got: %+v", out.Themes)
	}
}

func TestPreviewTool_Valid(t *testing.T) {
	session, ctx := newTestSession(t)

	res := callTool(t, session, ctx, "preview", map[string]any{"source": validSource})
	if res.IsError {
		t.Fatalf("expected success, got error result: %s", resultText(t, res))
	}

	html := resultText(t, res)
	if !strings.Contains(html, "<html") {
		t.Errorf("expected a self-contained HTML document, got: %.200s", html)
	}
	if !strings.Contains(html, "<style") {
		t.Errorf("expected embedded CSS (EmbedAssets forced true), got: %.200s", html)
	}
}

func TestPreviewTool_ValidTheme(t *testing.T) {
	session, ctx := newTestSession(t)

	res := callTool(t, session, ctx, "preview", map[string]any{"source": validSource, "theme": "academic"})
	if res.IsError {
		t.Fatalf("expected success for a known theme name, got error result: %s", resultText(t, res))
	}
}

// TestPreviewTool_RejectsPathLikeTheme es la misma regresión de ME-2
// (docs/SECURITY_AUDIT_2026-07.md) que slidelang guarda para su propio tool
// preview, reabierta acá por el mismo vector: el theme input es
// MCP-client-supplied, no el flag --theme del operador, así que nunca debe
// llegar a document.ThemeLoader.loadExternalTheme sin pasar primero por el
// guard de token opaco.
func TestPreviewTool_RejectsPathLikeTheme(t *testing.T) {
	session, ctx := newTestSession(t)

	for _, theme := range []string{"/etc/passwd", "../../../../etc/passwd", "C:\\Windows\\system.ini", "some/theme.json"} {
		res := callTool(t, session, ctx, "preview", map[string]any{"source": validSource, "theme": theme})
		if !res.IsError {
			t.Fatalf("theme %q: expected IsError=true, got success: %s", theme, resultText(t, res))
		}
		text := resultText(t, res)
		if !strings.Contains(text, "unknown theme") {
			t.Errorf("theme %q: expected an 'unknown theme' rejection, got: %s", theme, text)
		}
	}
}

func TestPreviewTool_TransformError(t *testing.T) {
	session, ctx := newTestSession(t)

	res := callTool(t, session, ctx, "preview", map[string]any{"source": duplicateLabelSource})
	if !res.IsError {
		t.Fatal("expected IsError=true: no HTML can be rendered when the transform stage fails")
	}

	var out previewError
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Error == "" {
		t.Error("expected a non-empty Error message for a transform-stage failure")
	}
}

// TestLintTool_RejectsOversizedInput es una regresión: sin
// util.CheckInputSize, un cliente MCP podía enviar un `source` sin límite
// directo al parser + normalizador AI (docs/SECURITY_AUDIT_2026-07.md, ME-8),
// a diferencia de `doclang build`, que siempre aplica
// --max-size/DOCLANG_MAX_SIZE.
func TestLintTool_RejectsOversizedInput(t *testing.T) {
	session, ctx := newTestSession(t)

	oversized := strings.Repeat("a", util.DefaultMaxInputBytes+1)
	res := callTool(t, session, ctx, "lint", map[string]any{"source": oversized})
	if !res.IsError {
		t.Fatal("expected IsError=true for input exceeding DefaultMaxInputBytes")
	}
	var out lintOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.Contains(out.Error, "too large") {
		t.Errorf("expected a size-limit error message, got: %q", out.Error)
	}
}

// TestLintTool_ConcurrentCalls es una prueba de cordura para el semáforo
// maxConcurrentParses: una ráfaga de llamadas concurrentes muy por encima de
// su capacidad nunca debe colgarse ni entrar en panic. La adquisición es
// deliberadamente no bloqueante (ver TestParseSource_FailsFastWhenSemaphoreFull
// para el motivo), así que bajo contención real algunas llamadas
// legítimamente reciben un resultado "server busy" en vez de encolarse — eso
// es el comportamiento correcto, no una falla.
func TestLintTool_ConcurrentCalls(t *testing.T) {
	session, ctx := newTestSession(t)

	const n = maxConcurrentParses * 3
	var wg sync.WaitGroup
	failures := make(chan string, n)
	busy := 0
	var busyMu sync.Mutex
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{
				Name:      "lint",
				Arguments: map[string]any{"source": validSource},
			})
			if err != nil {
				failures <- fmt.Sprintf("CallTool error: %v", err)
				return
			}
			if res.IsError {
				text := ""
				if len(res.Content) == 1 {
					if tc, ok := res.Content[0].(*sdkmcp.TextContent); ok {
						text = tc.Text
					}
				}
				if strings.Contains(text, "busy") {
					busyMu.Lock()
					busy++
					busyMu.Unlock()
					return
				}
				failures <- fmt.Sprintf("unexpected IsError result: %s", text)
			}
		}()
	}
	wg.Wait()
	close(failures)
	for f := range failures {
		t.Error(f)
	}
	t.Logf("%d/%d calls got a legitimate 'busy' rejection under burst concurrency", busy, n)
}

// TestParseSource_FailsFastWhenSemaphoreFull es una regresión: el
// adquirir del semáforo de parseSource nunca debe bloquear indefinidamente.
// Satura parseSemaphore directamente (bypaseando el parseo real) para
// simular deterministamente maxConcurrentParses parses genuinamente colgados
// para siempre, y confirma que una llamada subsiguiente a parseSource
// retorna un error claro de inmediato en vez de colgarse.
func TestParseSource_FailsFastWhenSemaphoreFull(t *testing.T) {
	for i := 0; i < maxConcurrentParses; i++ {
		parseSemaphore <- struct{}{}
	}
	defer func() {
		for i := 0; i < maxConcurrentParses; i++ {
			<-parseSemaphore
		}
	}()

	done := make(chan error, 1)
	go func() {
		_, _, err := parseSource(util.NewNoop(), validSource)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected a 'server busy' error when the semaphore is fully held, got nil")
		}
		if !strings.Contains(err.Error(), "busy") {
			t.Errorf("expected a busy-related error, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("parseSource blocked instead of failing fast when the semaphore was full -- this is exactly the permanent-DoS regression this test guards against")
	}
}
