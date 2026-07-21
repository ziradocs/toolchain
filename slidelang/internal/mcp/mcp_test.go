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
	"go.ziradocs.com/core/util"
)

const validSource = `---
title: "Test"
mode: flex
---

# Slide 1

Hello world.
`

// noFrontMatterSource carece del bloque "---" inicial que FrontMatterParser
// exige (ver CLAUDE.md) — produce un diagnóstico de error de parseo fatal,
// no un AST parcial.
const noFrontMatterSource = `# Slide 1

Hello world, no frontmatter.
`

// xrefSource trae una ecuación etiquetada y un \ref a esa misma etiqueta —
// prueba end-to-end de que parseSource es build-faithful (ver el fix en
// parse.go, issue #187/#189): la numeración y la resolución de \ref solo
// aparecen si la etapa de transform (transform.RunBuiltins + xref.Transform)
// corrió de verdad, no solo el parseo. Regresión directa del período en que
// este MCP quedó parse-only (creado 2026-07-14, #133; #239/#240 introdujeron
// la etapa de transform recién el 2026-07-19).
const xrefSource = `---
title: "Test"
mode: flex
---

# Slide 1

See \ref{eq:euler} below.

<<math>>
e^{i\pi} + 1 = 0
label: "eq:euler"
<<end>>
`

// newTestSession arma un servidor MCP real (con todos sus tools) y un
// cliente conectados vía el transporte in-memory del SDK — E2E real, no un
// mock: ejercita el mismo ListTools/CallTool que vería un cliente MCP real
// contra `slidelang mcp`.
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
	if out.SlideCount != 1 {
		t.Errorf("expected SlideCount=1, got %d", out.SlideCount)
	}
}

func TestLintTool_ParseError(t *testing.T) {
	session, ctx := newTestSession(t)

	res := callTool(t, session, ctx, "lint", map[string]any{"source": noFrontMatterSource})
	// lint no marca IsError por errores de contenido -- eso es precisamente
	// lo que el tool reporta como diagnóstico, no como fallo de la tool call.
	if res.IsError {
		t.Fatalf("lint should report content errors as diagnostics, not IsError: %s", resultText(t, res))
	}

	var out lintOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Valid {
		t.Fatal("expected Valid=false for content missing frontmatter")
	}
	found := false
	for _, d := range out.Diagnostics {
		if strings.Contains(d.Message, "FrontMatter") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a FrontMatter diagnostic, got: %+v", out.Diagnostics)
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

func TestGetASTTool_ParseError(t *testing.T) {
	session, ctx := newTestSession(t)

	res := callTool(t, session, ctx, "get_ast", map[string]any{"source": noFrontMatterSource})
	if !res.IsError {
		t.Fatal("expected IsError=true: no valid AST can be built from unparseable content")
	}

	var out getASTError
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Diagnostics) == 0 {
		t.Error("expected parse diagnostics in the error result")
	}
}

// TestGetASTTool_BuildFaithfulNumbering es la prueba central del fix de
// parseSource (issue #187/#189, converge con doclang/internal/mcp): si
// alguien revierte la etapa de transform a parse-only, este test falla.
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
		t.Fatal("expected at least the embedded themes (default, dark, minimal)")
	}
	names := make(map[string]bool, len(out.Themes))
	for _, th := range out.Themes {
		names[th.Name] = true
	}
	if !names["default"] {
		t.Errorf("expected embedded 'default' theme in list, got: %+v", out.Themes)
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

	res := callTool(t, session, ctx, "preview", map[string]any{"source": validSource, "theme": "dark"})
	if res.IsError {
		t.Fatalf("expected success for a known theme name, got error result: %s", resultText(t, res))
	}
}

// TestPreviewTool_RejectsPathLikeTheme is a regression test for ME-2
// (docs/SECURITY_AUDIT_2026-07.md) reopened via a new vector: the theme
// input is MCP-client-supplied, not the operator's own CLI flag, so it must
// never reach generator.resolveTheme's trusted=true raw-file-path shortcut
// (findAndLoadExternalTheme, loader.go:147) unvalidated. Any path-shaped
// value must be rejected before it can be os.Stat'd/loaded as an arbitrary
// local file.
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

func TestPreviewTool_ParseError(t *testing.T) {
	session, ctx := newTestSession(t)

	res := callTool(t, session, ctx, "preview", map[string]any{"source": noFrontMatterSource})
	if !res.IsError {
		t.Fatal("expected IsError=true: no HTML can be rendered from unparseable content")
	}

	var out previewError
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Diagnostics) == 0 {
		t.Error("expected parse diagnostics in the error result")
	}
}

// TestLintTool_RejectsOversizedInput is a regression test: parseSource
// originally ported only the CLI's timeout guard, not its size cap
// (util.CheckInputSize/DefaultMaxInputBytes), leaving MCP clients able to
// send an unbounded `source` straight into the parser + AI normalizer
// (docs/SECURITY_AUDIT_2026-07.md, ME-8's amplification concern) unlike
// `slidelang build`, which always enforces --max-size/SLIDELANG_MAX_SIZE.
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

// TestLintTool_ConcurrentCalls is a sanity check for the maxConcurrentParses
// semaphore added in parseSource: a burst of concurrent calls well beyond
// the semaphore's capacity must never hang or panic. The semaphore's
// acquire is intentionally non-blocking (see TestParseSource_FailsFastWhenSemaphoreFull
// for why), so under real contention some calls legitimately receive a
// "server busy" IsError result instead of being queued — that is the
// correct, intended behavior now, not a failure. Only a genuinely
// unexpected error (anything that isn't the busy message) is treated as a
// test failure.
//
// t.Fatalf is unsafe to call from a non-test goroutine, so this deliberately
// does not reuse the callTool/resultText helpers inside the spawned
// goroutines — it calls session.CallTool directly and reports failures via
// a channel drained on the test goroutine.
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

// TestParseSource_FailsFastWhenSemaphoreFull is a regression test for the
// permanent-DoS finding: parseSource's semaphore acquire must never block
// indefinitely. It directly saturates parseSemaphore (bypassing real
// parsing, to deterministically simulate maxConcurrentParses parses that
// are genuinely stuck forever -- the exact scenario a parser with no
// cancellation support can produce) and asserts a subsequent parseSource
// call returns a clear error immediately rather than hanging.
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
		_, _, err := parseSource(util.NewNoop(), validSource, "")
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
