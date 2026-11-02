// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package mcp

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/renderer/chromium"
	"go.ziradocs.com/core/v2/util"
)

// plainMarkdown no tiene frontmatter: es el caso que T1 dejó funcionando y
// el que más va a llegar desde un editor.
const plainMarkdown = "# Informe trimestral\n\nTexto sin frontmatter.\n\n## Detalle\n\nMás texto.\n"

const mermaidMarkdown = "# Con diagrama\n\n```mermaid\ngraph TD\n  A-->B\n```\n"

// exportFixture arma una raíz de export y, al lado (fuera de ella), un
// directorio "outside" para los intentos de escape.
type exportFixture struct {
	root    string
	outside string
}

func newExportFixture(t *testing.T) exportFixture {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := exportFixture{root: filepath.Join(base, "root"), outside: filepath.Join(base, "outside")}
	for _, d := range []string{f.root, f.outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f exportFixture) write(t *testing.T, rel, content string) string {
	t.Helper()
	p := filepath.Join(f.root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func newExportSession(t *testing.T, opts Options) (*sdkmcp.ClientSession, context.Context) {
	t.Helper()
	ctx := context.Background()
	server, err := NewServerWithOptions(util.NewNoop(), opts)
	if err != nil {
		t.Fatalf("NewServerWithOptions: %v", err)
	}
	clientTransport, serverTransport := sdkmcp.NewInMemoryTransports()
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

func exportOK(t *testing.T, s *sdkmcp.ClientSession, ctx context.Context, args map[string]any) exportOutput {
	t.Helper()
	res := callTool(t, s, ctx, "export", args)
	if res.IsError {
		t.Fatalf("export %v: unexpected error result: %s", args, resultText(t, res))
	}
	var out exportOutput
	if err := json.Unmarshal([]byte(resultText(t, res)), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

func exportFail(t *testing.T, s *sdkmcp.ClientSession, ctx context.Context, args map[string]any, wantSubstr string) exportError {
	t.Helper()
	res := callTool(t, s, ctx, "export", args)
	text := resultText(t, res)
	if !res.IsError {
		t.Fatalf("export %v: expected an error result, got success: %s", args, text)
	}
	var out exportError
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.Contains(out.Error, wantSubstr) {
		t.Fatalf("export %v: error %q does not contain %q", args, out.Error, wantSubstr)
	}
	return out
}

func chromiumAvailable() bool {
	_, err := chromium.NewChromiumManager("", false, &renderer.ChromiumLoggerAdapter{Logger: util.NewNoop()}).Detect()
	return err == nil
}

func zipContents(t *testing.T, path string) map[string][]byte {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open docx: %v", err)
	}
	defer zr.Close()
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		files[f.Name] = data
	}
	return files
}

// pngBytes genera un PNG de un color, para distinguir qué imagen terminó
// dentro del docx.
func pngBytes(t *testing.T, c color.Color) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 3, 3))
	for x := 0; x < 3; x++ {
		for y := 0; y < 3; y++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestListTools_IncludesExport(t *testing.T) {
	f := newExportFixture(t)
	s, ctx := newExportSession(t, Options{Root: f.root})
	got, err := s.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range got.Tools {
		if tool.Name == "export" {
			return
		}
	}
	t.Fatal("export tool not registered")
}

func TestExport_PlainMarkdownToDOCX(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "docs/informe.md", plainMarkdown)
	s, ctx := newExportSession(t, Options{Root: f.root})

	out := exportOK(t, s, ctx, map[string]any{"path": "docs/informe.md", "format": "docx"})
	if out.OutputPath != "docs/informe.docx" {
		t.Errorf("default output = %q, want docs/informe.docx", out.OutputPath)
	}
	if out.Format != "docx" {
		t.Errorf("format = %q", out.Format)
	}
	info, err := os.Stat(filepath.Join(f.root, "docs", "informe.docx"))
	if err != nil {
		t.Fatal(err)
	}
	if out.Bytes != info.Size() || out.Bytes == 0 {
		t.Errorf("bytes = %d, file size = %d", out.Bytes, info.Size())
	}
	doc := zipContents(t, filepath.Join(f.root, "docs", "informe.docx"))["word/document.xml"]
	if !bytes.Contains(doc, []byte("Texto sin frontmatter")) {
		t.Error("document.xml does not contain the body text")
	}
	leftovers, _ := filepath.Glob(filepath.Join(f.root, "docs", ".doclang-export-*"))
	if len(leftovers) != 0 {
		t.Errorf("temporary files left behind: %v", leftovers)
	}
}

func TestExport_HTML(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "a.md", plainMarkdown)
	s, ctx := newExportSession(t, Options{Root: f.root})

	out := exportOK(t, s, ctx, map[string]any{"path": "a.md", "format": "html", "theme": "technical", "toc": false})
	if out.OutputPath != "a.html" {
		t.Errorf("output = %q", out.OutputPath)
	}
	data, err := os.ReadFile(filepath.Join(f.root, "a.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("Informe trimestral")) {
		t.Error("html does not contain the title")
	}
}

func TestExport_PDF(t *testing.T) {
	if !chromiumAvailable() {
		t.Skip("Chromium not available")
	}
	f := newExportFixture(t)
	f.write(t, "a.md", plainMarkdown)
	s, ctx := newExportSession(t, Options{Root: f.root})

	out := exportOK(t, s, ctx, map[string]any{"path": "a.md", "format": "pdf"})
	data, err := os.ReadFile(filepath.Join(f.root, out.OutputPath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		t.Errorf("output is not a PDF: %q", data[:min(len(data), 16)])
	}
}

func TestExport_AbsolutePathInsideRootAndExplicitOutput(t *testing.T) {
	f := newExportFixture(t)
	in := f.write(t, "a.doclang", plainMarkdown)
	if err := os.MkdirAll(filepath.Join(f.root, "out"), 0o755); err != nil {
		t.Fatal(err)
	}
	s, ctx := newExportSession(t, Options{Root: f.root})

	out := exportOK(t, s, ctx, map[string]any{"path": in, "format": "html", "output": filepath.Join(f.root, "out", "final.html")})
	if out.OutputPath != "out/final.html" {
		t.Errorf("output = %q", out.OutputPath)
	}
}

func TestExport_Overwrite(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "a.md", plainMarkdown)
	existing := f.write(t, "a.html", "previous content")
	s, ctx := newExportSession(t, Options{Root: f.root})

	exportFail(t, s, ctx, map[string]any{"path": "a.md", "format": "html"}, "already exists")
	if data, _ := os.ReadFile(existing); string(data) != "previous content" {
		t.Fatalf("existing file was modified without overwrite: %q", data)
	}

	exportOK(t, s, ctx, map[string]any{"path": "a.md", "format": "html", "overwrite": true})
	if data, _ := os.ReadFile(existing); !bytes.Contains(data, []byte("Informe trimestral")) {
		t.Fatal("overwrite: true did not replace the file")
	}
}

func TestExport_LintErrorsReturnDiagnostics(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "empty.md", "---\ntitle: x\n---\n")
	s, ctx := newExportSession(t, Options{Root: f.root})

	out := exportFail(t, s, ctx, map[string]any{"path": "empty.md", "format": "html"}, "lint errors found")
	if len(out.Diagnostics) == 0 {
		t.Fatal("expected lint diagnostics in the error result")
	}
	if _, err := os.Stat(filepath.Join(f.root, "empty.html")); !os.IsNotExist(err) {
		t.Error("an output file was written despite lint errors")
	}
}

func TestExport_Rejections(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "ok.md", plainMarkdown)
	f.write(t, "notes.txt", plainMarkdown)
	if err := os.WriteFile(filepath.Join(f.outside, "secret.md"), []byte(plainMarkdown), 0o644); err != nil {
		t.Fatal(err)
	}
	// symlink dentro de la raíz que apunta afuera, para el input
	if err := os.Symlink(filepath.Join(f.outside, "secret.md"), filepath.Join(f.root, "link.md")); err != nil {
		t.Fatal(err)
	}
	// directorio-symlink dentro de la raíz que apunta afuera, para el output
	if err := os.Symlink(f.outside, filepath.Join(f.root, "outlink")); err != nil {
		t.Fatal(err)
	}
	// archivo de salida que es un symlink hacia afuera
	if err := os.Symlink(filepath.Join(f.outside, "victim.html"), filepath.Join(f.root, "victim.html")); err != nil {
		t.Fatal(err)
	}
	f.write(t, "inc.md", "# Doc\n\n@include ../outside/secret.md\n")

	s, ctx := newExportSession(t, Options{Root: f.root})
	cases := []struct {
		name string
		args map[string]any
		want string
	}{
		{"dotdot traversal", map[string]any{"path": "../outside/secret.md", "format": "docx"}, "escapes"},
		{"absolute outside root", map[string]any{"path": filepath.Join(f.outside, "secret.md"), "format": "docx"}, "outside the root"},
		{"symlink input escape", map[string]any{"path": "link.md", "format": "docx"}, "symlink"},
		{"symlink output dir escape", map[string]any{"path": "ok.md", "format": "html", "output": "outlink/x.html"}, "symlink"},
		{"symlink output file", map[string]any{"path": "ok.md", "format": "html", "output": "victim.html", "overwrite": true}, "symlink"},
		{"output dotdot", map[string]any{"path": "ok.md", "format": "html", "output": "../outside/x.html"}, "escapes"},
		{"output absolute outside", map[string]any{"path": "ok.md", "format": "html", "output": filepath.Join(f.outside, "x.html")}, "outside the root"},
		{"output wrong extension", map[string]any{"path": "ok.md", "format": "html", "output": "ok.md", "overwrite": true}, "extension"},
		{"include outside root", map[string]any{"path": "inc.md", "format": "html"}, "failed to expand includes"},
		{"disallowed extension", map[string]any{"path": "notes.txt", "format": "html"}, "unsupported input extension"},
		{"directory as input", map[string]any{"path": "dir.md", "format": "html"}, "not a regular file"},
		{"invalid theme", map[string]any{"path": "ok.md", "format": "html", "theme": "../../etc/passwd"}, "unknown theme"},
		{"invalid format", map[string]any{"path": "ok.md", "format": "markdown"}, "unsupported format"},
	}
	if err := os.Mkdir(filepath.Join(f.root, "dir.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exportFail(t, s, ctx, tc.args, tc.want)
		})
	}

	entries, _ := os.ReadDir(f.outside)
	for _, e := range entries {
		if e.Name() != "secret.md" {
			t.Errorf("a file was written outside the root: %s", e.Name())
		}
	}
}

// TestExport_ImageOutsideRootIsBlocked: una imagen fuera de la raíz no
// falla el export (el generador DOCX deja un marcador "[Image blocked: …]",
// AL-4), pero sus bytes no llegan al docx. La imagen dentro de la raíz sí
// entra, para que la prueba negativa no pase por vacía.
//
// Las rutas empiezan con assets/ porque el normalizador del modo flex
// reescribe cualquier otra ruta de imagen a assets/images/<nombre>, lo que
// ya descarta el ".." y la ruta absoluta antes de llegar al generador; con
// el prefijo la ruta llega intacta y la prueba ejercita el confinamiento.
func TestExport_ImageOutsideRootIsBlocked(t *testing.T) {
	f := newExportFixture(t)
	secret := pngBytes(t, color.RGBA{R: 200, G: 10, B: 10, A: 255})
	inside := pngBytes(t, color.RGBA{R: 10, G: 200, B: 10, A: 255})
	if err := os.WriteFile(filepath.Join(f.outside, "secret.png"), secret, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(f.root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, "assets", "inside.png"), inside, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(f.outside, "secret.png"), filepath.Join(f.root, "assets", "link.png")); err != nil {
		t.Fatal(err)
	}
	f.write(t, "img.md", "# Imágenes\n\n![ok](assets/inside.png)\n\n![rel](assets/../../outside/secret.png)\n\n![link](assets/link.png)\n")
	s, ctx := newExportSession(t, Options{Root: f.root})

	out := exportOK(t, s, ctx, map[string]any{"path": "img.md", "format": "docx"})
	files := zipContents(t, filepath.Join(f.root, out.OutputPath))
	foundInside := false
	for name, data := range files {
		if bytes.Equal(data, secret) {
			t.Errorf("image from outside the root was embedded as %s", name)
		}
		if bytes.Equal(data, inside) {
			foundInside = true
		}
	}
	if !foundInside {
		t.Error("image inside the root was not embedded (control)")
	}
	if !bytes.Contains(files["word/document.xml"], []byte("Image blocked")) {
		t.Error("expected an [Image blocked: …] placeholder in document.xml")
	}
}

func TestExport_ChromiumMissing(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "a.md", plainMarkdown)
	f.write(t, "diagram.md", mermaidMarkdown)
	s, ctx := newExportSession(t, Options{Root: f.root, ChromiumPath: filepath.Join(f.root, "no-such-chromium")})

	out := exportFail(t, s, ctx, map[string]any{"path": "a.md", "format": "pdf"}, "Chromium is required for PDF output")
	if !strings.Contains(out.Hint, "doclang build --install-chromium") {
		t.Errorf("hint = %q", out.Hint)
	}
	if len(out.RequiresChromium) != 1 {
		t.Errorf("requiresChromium = %v", out.RequiresChromium)
	}

	out = exportFail(t, s, ctx, map[string]any{"path": "diagram.md", "format": "docx"}, "mermaid diagrams in DOCX")
	if out.Hint == "" {
		t.Error("missing hint")
	}

	// Sin diagramas, DOCX y HTML no necesitan Chromium aunque no exista.
	exportOK(t, s, ctx, map[string]any{"path": "a.md", "format": "docx"})
	exportOK(t, s, ctx, map[string]any{"path": "diagram.md", "format": "html"})

	if _, err := os.Stat(filepath.Join(f.root, "a.pdf")); !os.IsNotExist(err) {
		t.Error("a PDF was written without Chromium")
	}
}

func TestExport_Timeout(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "a.md", plainMarkdown)
	s, ctx := newExportSession(t, Options{Root: f.root, ExportTimeout: time.Nanosecond})

	exportFail(t, s, ctx, map[string]any{"path": "a.md", "format": "docx"}, "timed out")

	// El trabajo abandonado termina por su cuenta y borra su temporal; el
	// destino nunca aparece.
	deadline := time.Now().Add(20 * time.Second)
	for {
		left, _ := filepath.Glob(filepath.Join(f.root, ".doclang-export-*"))
		if len(left) == 0 && len(exportSemaphore) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("temporary files not cleaned up: %v", left)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(filepath.Join(f.root, "a.docx")); !os.IsNotExist(err) {
		t.Error("output written despite the timeout")
	}
}

func TestExport_ConcurrencyLimit(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "a.md", plainMarkdown)
	s, ctx := newExportSession(t, Options{Root: f.root})

	for i := 0; i < maxConcurrentExports; i++ {
		exportSemaphore <- struct{}{}
	}
	t.Cleanup(func() {
		for i := 0; i < maxConcurrentExports; i++ {
			<-exportSemaphore
		}
	})
	exportFail(t, s, ctx, map[string]any{"path": "a.md", "format": "html"}, "server busy")
}

func TestNewServerWithOptions_InvalidRoot(t *testing.T) {
	if _, err := NewServerWithOptions(util.NewNoop(), Options{Root: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("expected an error for a root that does not exist")
	}
}
