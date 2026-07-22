// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

// minimalPNG is a valid 1x1 transparent PNG, small enough to embed inline.
var minimalPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
	0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
	0x0d, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x64, 0x60, 0x60, 0x60,
	0x60, 0x00, 0x00, 0x00, 0x05, 0x00, 0x01, 0x5a, 0x67, 0x35, 0x5f, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
}

func astWithImage(source string) *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	doc.FrontMatter = ast.NewFrontMatterNode(pos)
	doc.FrontMatter.Title = "Image Confinement Test"

	block := ast.NewContentBlock(diagnostics.NewPosition(2, 1), "content")
	block.Title = "Section"
	block.Elements = append(block.Elements, ast.NewImageElement(diagnostics.NewPosition(3, 1), source, "alt text"))
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	return doc
}

// docxMediaFiles unzips a .docx and returns the raw bytes of every file under word/media/.
func docxMediaFiles(t *testing.T, docxPath string) map[string][]byte {
	t.Helper()
	r, err := zip.OpenReader(docxPath)
	if err != nil {
		t.Fatalf("failed to open generated docx: %v", err)
	}
	defer func() { _ = r.Close() }()

	media := make(map[string][]byte)
	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, "word/media/") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("failed to open %s in docx: %v", f.Name, err)
		}
		buf, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("failed to read %s in docx: %v", f.Name, err)
		}
		media[f.Name] = buf
	}
	return media
}

// TestDOCX_ImageConfinement_RejectsAbsolutePath es una regresión para
// docs/SECURITY_AUDIT_2026-07.md AL-4: una fuente de imagen absoluta fuera
// del AssetRoot no debe copiarse a word/media/ del .docx generado.
func TestDOCX_ImageConfinement_RejectsAbsolutePath(t *testing.T) {
	secretDir := t.TempDir()
	secretPath := filepath.Join(secretDir, "secret.png")
	secretMarker := []byte("THIS-IS-SECRET-CONTENT-NOT-A-REAL-PNG")
	if err := os.WriteFile(secretPath, secretMarker, 0644); err != nil {
		t.Fatal(err)
	}

	assetRoot := t.TempDir()
	logger := newTestLogger()
	gen := New(logger)
	doc := astWithImage(secretPath) // ruta absoluta, fuera de assetRoot
	output := filepath.Join(t.TempDir(), "out.docx")

	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx", AssetRoot: assetRoot}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	for name, content := range docxMediaFiles(t, output) {
		if strings.Contains(string(content), "SECRET") {
			t.Fatalf("secret file content leaked into %s of the generated docx", name)
		}
	}

	blocked := false
	for _, w := range logger.warns {
		if strings.Contains(w, "blocked") {
			blocked = true
		}
	}
	if !blocked {
		t.Errorf("expected a 'blocked' warning log, got: %#v", logger.warns)
	}
}

// TestDOCX_ImageConfinement_RejectsTraversal cubre el caso "../../secret.png"
// del criterio de aceptación de AL-4.
func TestDOCX_ImageConfinement_RejectsTraversal(t *testing.T) {
	parentDir := t.TempDir()
	secretMarker := []byte("THIS-IS-SECRET-CONTENT-NOT-A-REAL-PNG")
	if err := os.WriteFile(filepath.Join(parentDir, "secret.png"), secretMarker, 0644); err != nil {
		t.Fatal(err)
	}

	assetRoot := filepath.Join(parentDir, "docs")
	if err := os.MkdirAll(assetRoot, 0755); err != nil {
		t.Fatal(err)
	}

	logger := newTestLogger()
	gen := New(logger)
	doc := astWithImage("../secret.png")
	output := filepath.Join(t.TempDir(), "out.docx")

	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx", AssetRoot: assetRoot}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	for name, content := range docxMediaFiles(t, output) {
		if strings.Contains(string(content), "SECRET") {
			t.Fatalf("secret file content leaked into %s of the generated docx", name)
		}
	}
}

// TestDOCX_ImageConfinement_AllowsImageWithinRoot confirma que una imagen
// legítima dentro del AssetRoot se sigue embebiendo sin regresión.
func TestDOCX_ImageConfinement_AllowsImageWithinRoot(t *testing.T) {
	assetRoot := t.TempDir()
	imgPath := filepath.Join(assetRoot, "logo.png")
	if err := os.WriteFile(imgPath, minimalPNG, 0644); err != nil {
		t.Fatal(err)
	}

	logger := newTestLogger()
	gen := New(logger)
	doc := astWithImage("logo.png")
	output := filepath.Join(t.TempDir(), "out.docx")

	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx", AssetRoot: assetRoot}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	media := docxMediaFiles(t, output)
	if len(media) == 0 {
		t.Fatal("expected the legitimate image to be embedded in word/media/, found none")
	}
}

// TestDOCX_ImageConfinement_EmptyAssetRootStillConfines es una regresión
// encontrada en code-review: GeneratorOptions{} sin AssetRoot explícito
// (p. ej. un consumidor directo de la librería, no vía build.go, que
// siempre resuelve uno) no debe reabrir AL-4 silenciosamente — se confina
// al directorio de trabajo actual en vez de desactivar la confinación.
func TestDOCX_ImageConfinement_EmptyAssetRootStillConfines(t *testing.T) {
	secretDir := t.TempDir()
	secretPath := filepath.Join(secretDir, "secret.png")
	secretMarker := []byte("THIS-IS-SECRET-CONTENT-NOT-A-REAL-PNG")
	if err := os.WriteFile(secretPath, secretMarker, 0644); err != nil {
		t.Fatal(err)
	}

	logger := newTestLogger()
	gen := New(logger)
	doc := astWithImage(secretPath) // ruta absoluta, fuera del cwd
	output := filepath.Join(t.TempDir(), "out.docx")

	// Nota: NO se configura AssetRoot en GeneratorOptions.
	if err := gen.Generate(doc, output, GeneratorOptions{Format: "docx"}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	for name, content := range docxMediaFiles(t, output) {
		if strings.Contains(string(content), "SECRET") {
			t.Fatalf("secret file content leaked into %s despite no explicit AssetRoot", name)
		}
	}
}
