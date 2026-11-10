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
	"runtime"
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
	defer func() { _ = zr.Close() }()
	files := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		_ = rc.Close()
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
	// Marcador localizado (sin lang: español), sin la ruta de fuera de la raíz.
	if !bytes.Contains(files["word/document.xml"], []byte("Imagen no disponible")) {
		t.Error("expected an [Imagen no disponible: …] placeholder in document.xml")
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

func TestCleanupPendingExports(t *testing.T) {
	p := filepath.Join(t.TempDir(), ".doclang-export-test.docx")
	if err := os.WriteFile(p, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	trackTemp(p)
	CleanupPendingExports()
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("pending temporary file not removed: %v", err)
	}
	pendingTemps.Lock()
	defer pendingTemps.Unlock()
	if len(pendingTemps.paths) != 0 {
		t.Fatalf("pending set not emptied: %v", pendingTemps.paths)
	}
}

func TestNewServerWithOptions_InvalidRoot(t *testing.T) {
	if _, err := NewServerWithOptions(util.NewNoop(), Options{Root: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("expected an error for a root that does not exist")
	}
}

// TestExport_ImagesResolveFromSourceFile: un .md en un subdirectorio de la
// raíz con imágenes al lado, en un subdirectorio y en un hermano. Se
// resuelven contra el archivo, como en build, y siguen confinadas a --root:
// un "../" que sale de la raíz no entra.
func TestExport_ImagesResolveFromSourceFile(t *testing.T) {
	f := newExportFixture(t)
	images := map[string][]byte{
		"proj/logo.png":  pngBytes(t, color.RGBA{R: 255, A: 255}),
		"proj/img/a.png": pngBytes(t, color.RGBA{G: 255, A: 255}),
		"shared/b.png":   pngBytes(t, color.RGBA{B: 255, A: 255}),
	}
	for rel, data := range images {
		p := filepath.Join(f.root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	secret := pngBytes(t, color.RGBA{R: 200, G: 200, A: 255})
	if err := os.WriteFile(filepath.Join(f.outside, "secret.png"), secret, 0o644); err != nil {
		t.Fatal(err)
	}
	f.write(t, "proj/README.md", "# Proyecto\n\n![l](logo.png)\n\n![a](./img/a.png)\n\n![b](../shared/b.png)\n\n![s](../../outside/secret.png)\n")
	s, ctx := newExportSession(t, Options{Root: f.root})

	out := exportOK(t, s, ctx, map[string]any{"path": "proj/README.md", "format": "docx"})
	files := zipContents(t, filepath.Join(f.root, out.OutputPath))
	for rel, want := range images {
		found := false
		for _, data := range files {
			if bytes.Equal(data, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s no está embebida en el DOCX", rel)
		}
	}
	for name, data := range files {
		if bytes.Equal(data, secret) {
			t.Errorf("una imagen fuera de --root se embebió como %s", name)
		}
	}
}

// TestExport_HungChromiumReleasesSlot: un Chromium que nunca responde no se
// queda con el slot del semáforo. Antes los generadores corrían con
// context.Background(): dos exports colgados dejaban `export` respondiendo
// "server busy" hasta reiniciar. Con el contexto del export propagado, al
// vencer el timeout el navegador se mata y el slot se libera.
func TestExport_HungChromiumReleasesSlot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("usa un script de shell como Chromium falso")
	}
	f := newExportFixture(t)
	f.write(t, "a.md", plainMarkdown)
	fake := filepath.Join(t.TempDir(), "hung-chrome")
	// Contesta --version (core.ChromiumManager.GetVersion lo llama sin
	// contexto) y se cuelga en todo lo demás, como un navegador que nunca
	// abre su puerto de DevTools.
	script := "#!/bin/sh\nif [ \"$1\" = --version ]; then echo 'Fake Chrome 1.0'; exit 0; fi\nexec sleep 600\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	s, ctx := newExportSession(t, Options{Root: f.root, ChromiumPath: fake, ExportTimeout: 500 * time.Millisecond})

	exportFail(t, s, ctx, map[string]any{"path": "a.md", "format": "pdf"}, "timed out")

	deadline := time.Now().Add(15 * time.Second)
	for len(exportSemaphore) != 0 {
		if time.Now().After(deadline) {
			t.Fatal("el slot del export colgado no se liberó tras el timeout")
		}
		time.Sleep(20 * time.Millisecond)
	}
	left, _ := filepath.Glob(filepath.Join(f.root, ".doclang-export-*"))
	if len(left) != 0 {
		t.Errorf("temporales sin borrar: %v", left)
	}
}

// TestExport_IncludeResolvesLikeBuild: `chap/main.md` con `@include part.md`
// toma el part.md hermano, como `doclang build`, y no el de la raíz. Antes
// export resolvía los @include contra --root.
func TestExport_IncludeResolvesLikeBuild(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "part.md", "Texto de la RAIZ.\n")
	f.write(t, "chap/part.md", "Texto del CAPITULO.\n")
	f.write(t, "chap/main.md", "# Capítulo\n\n@include part.md\n")
	s, ctx := newExportSession(t, Options{Root: f.root})

	out := exportOK(t, s, ctx, map[string]any{"path": "chap/main.md", "format": "html"})
	data, err := os.ReadFile(filepath.Join(f.root, out.OutputPath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("CAPITULO")) || bytes.Contains(data, []byte("RAIZ")) {
		t.Errorf("export no tomó el part.md hermano")
	}

	// Un @include que sale del directorio del archivo se rechaza, como en
	// build sin --include-root.
	f.write(t, "chap/escape.md", "# E\n\n@include ../part.md\n")
	exportFail(t, s, ctx, map[string]any{"path": "chap/escape.md", "format": "html"}, "include")
}

// TestExport_GeneratorWarningsReachDiagnostics: una imagen que existe pero
// no se puede insertar sale como diagnóstico del resultado. Antes el logger
// del servidor era un no-op y el resultado decía OK con diagnostics: [].
func TestExport_GeneratorWarningsReachDiagnostics(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "broken.png", "esto no es un PNG")
	f.write(t, "a.md", "# Imagen\n\n![rota](broken.png)\n")
	s, ctx := newExportSession(t, Options{Root: f.root})

	out := exportOK(t, s, ctx, map[string]any{"path": "a.md", "format": "docx"})
	found := false
	for _, d := range out.Diagnostics {
		if d.Source == "generator" && strings.Contains(d.Message, "broken.png") && strings.Contains(d.Message, "could not be inserted") {
			found = true
			if strings.Contains(d.Message, f.root) {
				t.Errorf("el diagnóstico expone la raíz absoluta: %q", d.Message)
			}
			if d.Position.Line != 3 {
				t.Errorf("el diagnóstico no trae la línea de la imagen: %+v", d.Position)
			}
		}
	}
	if !found {
		t.Errorf("falta el diagnóstico de broken.png: %+v", out.Diagnostics)
	}
}

// TestExportRoot_RejectsFilesystemRoot: --root / no confina nada, así que
// el servidor no arranca con ella y pide un directorio de proyecto.
func TestExportRoot_RejectsFilesystemRoot(t *testing.T) {
	root := string(filepath.Separator)
	if runtime.GOOS == "windows" {
		root = filepath.VolumeName(os.TempDir()) + `\`
	}
	_, err := newExportRoot(root)
	if err == nil || !strings.Contains(err.Error(), "choose a project directory") {
		t.Fatalf("newExportRoot(%q) = %v, want a filesystem-root error", root, err)
	}
	if _, err := NewServerWithOptions(util.NewNoop(), Options{Root: root}); err == nil {
		t.Error("NewServerWithOptions aceptó la raíz del sistema de archivos")
	}
}

// TestExport_IncludeRootArgument: includeRoot es el --include-root de build.
// Con "." un @include relativo a la raíz (shared/x.md desde chap/main.md)
// funciona; fuera de --root se rechaza.
func TestExport_IncludeRootArgument(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "shared/x.md", "Texto COMPARTIDO.\n")
	f.write(t, "chap/main.md", "# Capítulo\n\n@include shared/x.md\n")
	s, ctx := newExportSession(t, Options{Root: f.root})

	// Sin includeRoot se resuelve contra chap/ y no existe.
	exportFail(t, s, ctx, map[string]any{"path": "chap/main.md", "format": "html"}, "include")

	out := exportOK(t, s, ctx, map[string]any{"path": "chap/main.md", "format": "html", "includeRoot": "."})
	data, err := os.ReadFile(filepath.Join(f.root, out.OutputPath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("COMPARTIDO")) {
		t.Error("includeRoot no se usó para resolver el @include")
	}

	for _, bad := range []string{"..", f.outside, "noexiste", "chap/main.md"} {
		exportFail(t, s, ctx, map[string]any{"path": "chap/main.md", "format": "html", "includeRoot": bad, "overwrite": true}, "invalid includeRoot")
	}
}

// Los diagnósticos de un archivo incluido llevan su archivo, relativo al
// documento y sin la raíz absoluta, y su línea en ese archivo: los del
// linter y también los warnings del generador, con el mensaje diciendo lo
// mismo que la posición.
func TestExport_IncludedDiagnosticsPointAtSource(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "broken.png", "esto no es un PNG")
	f.write(t, "shared/x.md", "## X\n\n```\n```\n\n![rota](broken.png)\n")
	f.write(t, "chap/main.md", "# Capítulo\n\nUno.\n\nDos.\n\n@include shared/x.md\n")
	s, ctx := newExportSession(t, Options{Root: f.root})

	out := exportOK(t, s, ctx, map[string]any{"path": "chap/main.md", "format": "docx", "includeRoot": "."})
	var code001, image bool
	for _, d := range out.Diagnostics {
		if strings.Contains(d.File, f.root) {
			t.Errorf("File exposes the absolute root: %q", d.File)
		}
		if d.RuleID == "CODE001" {
			code001 = true
			if d.File != "../shared/x.md" || d.Position.Line != 3 {
				t.Errorf("CODE001 at %s:%d, want ../shared/x.md:3", d.File, d.Position.Line)
			}
		}
		if d.Source == "generator" && strings.Contains(d.Message, "broken.png") {
			image = true
			if d.File != "../shared/x.md" || d.Position.Line != 6 {
				t.Errorf("image warning at %s:%d, want ../shared/x.md:6", d.File, d.Position.Line)
			}
			if !strings.Contains(d.Message, "../shared/x.md:6") || strings.Contains(d.Message, "line ") {
				t.Errorf("message does not say where the image is: %q", d.Message)
			}
		}
	}
	if !code001 || !image {
		t.Errorf("missing CODE001 (%v) or the image warning (%v): %+v", code001, image, out.Diagnostics)
	}
}

func TestRedactGenerator(t *testing.T) {
	f := newExportFixture(t)
	root, err := newExportRoot(f.root)
	if err != nil {
		t.Fatal(err)
	}
	cfg := exportConfig{root: root, chromiumPath: "/opt/secret/chrome"}
	tmp := filepath.Join(os.TempDir(), "doclang-docx-123", "math_ab.png")
	msg := cfg.redactGenerator("failed " + tmp + " with /opt/secret/chrome in " + filepath.Join(f.root, "a.md"))
	for _, leak := range []string{os.TempDir(), "/opt/secret", f.root} {
		if strings.Contains(msg, leak) {
			t.Errorf("%q sigue en %q", leak, msg)
		}
	}
}

// TestExport_BaseDirArgument: una copia temporal de docs/guide.md, guardada
// en otro directorio de la raíz, exporta con las imágenes de docs/ y con
// sus @include resueltos desde ahí. baseDir fuera de la raíz, inexistente
// o que no es un directorio se rechaza.
func TestExport_BaseDirArgument(t *testing.T) {
	f := newExportFixture(t)
	logo := pngBytes(t, color.RGBA{R: 255, A: 255})
	if err := os.MkdirAll(filepath.Join(f.root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, "docs", "logo.png"), logo, 0o644); err != nil {
		t.Fatal(err)
	}
	f.write(t, "docs/part.md", "Texto de la PARTE.\n")
	f.write(t, ".tmp/guide.md", "# Guía\n\n![l](logo.png)\n\n@include part.md\n")
	s, ctx := newExportSession(t, Options{Root: f.root})

	// Sin baseDir el @include se busca en .tmp/ y no existe.
	exportFail(t, s, ctx, map[string]any{"path": ".tmp/guide.md", "format": "docx"}, "include")

	out := exportOK(t, s, ctx, map[string]any{"path": ".tmp/guide.md", "format": "docx", "baseDir": "docs"})
	files := zipContents(t, filepath.Join(f.root, out.OutputPath))
	found := false
	for _, data := range files {
		if bytes.Equal(data, logo) {
			found = true
		}
	}
	if !found {
		t.Error("logo.png no se resolvió desde baseDir")
	}
	if !bytes.Contains(files["word/document.xml"], []byte("PARTE")) {
		t.Error("el @include no se resolvió desde baseDir")
	}

	for _, bad := range []string{"..", f.outside, "noexiste", "docs/logo.png"} {
		exportFail(t, s, ctx, map[string]any{"path": ".tmp/guide.md", "format": "docx", "baseDir": bad, "overwrite": true}, "invalid baseDir")
	}
}

// TestExportRoot_ResolveDirPinsRealPath: baseDir/includeRoot pedidos por un
// symlink dentro de la raíz se devuelven ya resueltos. Si después de validar
// el symlink se cambia para apuntar fuera de --root, el directorio que usa
// el export sigue siendo el validado, dentro de la raíz.
func TestExportRoot_ResolveDirPinsRealPath(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "docs/part.md", "Texto de DOCS.\n")
	if err := os.WriteFile(filepath.Join(f.outside, "part.md"), []byte("Texto de AFUERA.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(f.root, "link")
	if err := os.Symlink(filepath.Join(f.root, "docs"), link); err != nil {
		t.Skipf("symlinks no disponibles: %v", err)
	}
	root, err := newExportRoot(f.root)
	if err != nil {
		t.Fatal(err)
	}

	dir, err := root.resolveDir("link")
	if err != nil {
		t.Fatal(err)
	}
	if dir != filepath.Join(root.dir, "docs") {
		t.Fatalf("resolveDir(link) = %q, quería la ruta real %q", dir, filepath.Join(root.dir, "docs"))
	}

	// Se cambia el symlink para que apunte fuera de la raíz.
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(f.outside, link); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "part.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("DOCS")) {
		t.Error("el directorio validado siguió al symlink fuera de la raíz")
	}
	// Y pedido de nuevo, el symlink que ahora sale de la raíz se rechaza.
	if _, err := root.resolveDir("link"); err == nil {
		t.Error("un symlink que apunta fuera de --root debería rechazarse")
	}

	// De punta a punta: baseDir por el symlink resuelve el @include desde
	// el destino real.
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(f.root, "docs"), link); err != nil {
		t.Fatal(err)
	}
	f.write(t, ".tmp/sel.md", "# Sel\n\n@include part.md\n")
	s, ctx := newExportSession(t, Options{Root: f.root})
	out := exportOK(t, s, ctx, map[string]any{"path": ".tmp/sel.md", "format": "html", "baseDir": "link"})
	html, err := os.ReadFile(filepath.Join(f.root, out.OutputPath))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(html, []byte("DOCS")) {
		t.Error("el @include no se resolvió desde el destino real de baseDir")
	}
}

// TestExport_AllowRemoteImagesNeedsServerFlag: allowRemoteImages lo pone el
// cliente, así que solo vale si el operador arrancó el servidor con
// --allow-remote-images; si no, se ignora con un aviso en los diagnósticos.
func TestExport_AllowRemoteImagesNeedsServerFlag(t *testing.T) {
	for _, serverAllows := range []bool{false, true} {
		f := newExportFixture(t)
		f.write(t, "a.md", "# A\n\nTexto.\n")
		s, ctx := newExportSession(t, Options{Root: f.root, AllowRemoteImages: serverAllows})
		out := exportOK(t, s, ctx, map[string]any{"path": "a.md", "format": "docx", "allowRemoteImages": true})
		warned := false
		for _, d := range out.Diagnostics {
			if strings.Contains(d.Message, "allowRemoteImages ignored") {
				warned = true
			}
		}
		if warned == serverAllows {
			t.Errorf("server flag %v: warning = %v, want %v", serverAllows, warned, !serverAllows)
		}
	}
}

// TestExport_SVGOnlyInCodeIsNotAnImage: un tutorial cuya única mención de un
// SVG está en un bloque de código o un code span no pide Chromium para el
// DOCX; uno con una imagen SVG de verdad sí.
func TestExport_SVGOnlyInCodeIsNotAnImage(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "tut.md", "# Tutorial\n\nEscribe una imagen así: `![x](a.svg)`.\n\n```markdown\n![Diagrama](diagrama.svg)\n```\n")
	f.write(t, "real.md", "# Real\n\n![Diagrama](diagrama.svg)\n")
	s, ctx := newExportSession(t, Options{Root: f.root, ChromiumPath: filepath.Join(f.root, "no-such-chromium")})
	exportOK(t, s, ctx, map[string]any{"path": "tut.md", "format": "docx"})
	exportFail(t, s, ctx, map[string]any{"path": "real.md", "format": "docx"}, "SVG images in DOCX")
}

// Un mermaid dentro de un bloque especial o una ecuación dentro de una
// columna tipada también exige
// Chromium: el chequeo previo miraba solo el primer nivel de la sección y el
// DOCX salía con un marcador de no disponible, sin error.
func TestExport_ChromiumMissingNestedRichElements(t *testing.T) {
	f := newExportFixture(t)
	f.write(t, "callout-mermaid.md", "# Doc\n\n:::info Nota\nTexto.\n\n```mermaid\ngraph TD\n  A-->B\n```\n:::\n")
	f.write(t, "column-math.md", "# Doc\n\n::: grid\n::: column typed\n$$\nE = mc^2\n$$\n:::\n::: column\nx\n:::\n:::\n")
	s, ctx := newExportSession(t, Options{Root: f.root, ChromiumPath: filepath.Join(f.root, "no-such-chromium")})

	exportFail(t, s, ctx, map[string]any{"path": "callout-mermaid.md", "format": "docx"}, "mermaid diagrams in DOCX")
	exportFail(t, s, ctx, map[string]any{"path": "column-math.md", "format": "docx"}, "math equations in DOCX")
	if _, err := os.Stat(filepath.Join(f.root, "callout-mermaid.docx")); !os.IsNotExist(err) {
		t.Error("a DOCX was written without Chromium")
	}
}

// TestExport_IncludedImagesResolveFromTheirFile: una imagen escrita en un
// archivo incluido de otra carpeta se resuelve desde esa carpeta, igual que
// en `doclang build`.
func TestExport_IncludedImagesResolveFromTheirFile(t *testing.T) {
	f := newExportFixture(t)
	img := pngBytes(t, color.RGBA{G: 255, A: 255})
	if err := os.MkdirAll(filepath.Join(f.root, "docs", "a"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.root, "docs", "a", "u.png"), img, 0o644); err != nil {
		t.Fatal(err)
	}
	f.write(t, "docs/a/uno.md", "## Uno\n\n![U](u.png)\n")
	f.write(t, ".tmp/sel.md", "# Selección\n\n@include a/uno.md\n")
	s, ctx := newExportSession(t, Options{Root: f.root})

	out := exportOK(t, s, ctx, map[string]any{"path": ".tmp/sel.md", "format": "docx", "baseDir": "docs"})
	found := false
	for _, data := range zipContents(t, filepath.Join(f.root, out.OutputPath)) {
		if bytes.Equal(data, img) {
			found = true
		}
	}
	if !found {
		t.Error("la imagen del archivo incluido no se resolvió desde su carpeta")
	}
}
