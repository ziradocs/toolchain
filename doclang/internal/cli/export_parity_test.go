// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package cli

import (
	"archive/zip"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.ziradocs.com/core/v2/util"
	"go.ziradocs.com/doclang/v2/internal/mcp"
)

// TestBuildAndMCPExportProduceTheSameOutput: `doclang build` y el tool
// `export` del MCP comparten internal/pipeline, así que el mismo documento
// sale igual por los dos caminos. En HTML solo cambia el nonce de la CSP
// (aleatorio en cada render); en DOCX se compara word/document.xml, porque
// docProps/core.xml lleva la hora.
func TestBuildAndMCPExportProduceTheSameOutput(t *testing.T) {
	fixtures := map[string]string{
		"plain.md":      "# Informe\n\nSin frontmatter.\n\n## Detalle\n\nTexto.\n",
		"front.doclang": "---\ntitle: \"Con frontmatter\"\ntoc: false\n---\n\n# Uno\n\nTexto **fuerte**.\n\n# Dos\n\n- a\n- b\n",
		"numbered.md":   "---\ntitle: \"Numerado\"\nnumbering: true\n---\n\n# Primero\n\nUno.\n\n# Segundo\n\nDos.\n",
	}
	nonce := regexp.MustCompile(`nonce-[A-Za-z0-9+/=]+|nonce="[^"]+"`)

	for name, content := range fixtures {
		for _, format := range []string{"html", "docx"} {
			t.Run(name+"/"+format, func(t *testing.T) {
				dir, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				in := filepath.Join(dir, name)
				if err := os.WriteFile(in, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}

				buildOut := filepath.Join(dir, "build")
				cmd := NewBuildCommand(nil, nil, nil, nil, nil)
				cmd.SetArgs([]string{in, "--format", format, "--output", buildOut, "--log-level", "error"})
				cmd.SilenceUsage = true
				cmd.SilenceErrors = true
				if err := cmd.Execute(); err != nil {
					t.Fatalf("build: %v", err)
				}
				base := name[:len(name)-len(filepath.Ext(name))] + "." + format
				fromBuild := readOutput(t, filepath.Join(buildOut, base), format)

				server, err := mcp.NewServerWithOptions(util.NewNoop(), mcp.Options{Root: dir})
				if err != nil {
					t.Fatal(err)
				}
				ct, st := sdkmcp.NewInMemoryTransports()
				ctx := context.Background()
				if _, err := server.Connect(ctx, st, nil); err != nil {
					t.Fatal(err)
				}
				session, err := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "t", Version: "0"}, nil).Connect(ctx, ct, nil)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = session.Close() }()
				res, err := session.CallTool(ctx, &sdkmcp.CallToolParams{Name: "export", Arguments: map[string]any{"path": name, "format": format}})
				if err != nil {
					t.Fatal(err)
				}
				if res.IsError {
					t.Fatalf("export: %v", res.Content[0].(*sdkmcp.TextContent).Text)
				}
				fromExport := readOutput(t, filepath.Join(dir, base), format)

				if format == "html" {
					fromBuild = nonce.ReplaceAll(fromBuild, []byte("nonce-X"))
					fromExport = nonce.ReplaceAll(fromExport, []byte("nonce-X"))
				}
				if string(fromBuild) != string(fromExport) {
					t.Errorf("build and export differ for %s (%d vs %d bytes)", base, len(fromBuild), len(fromExport))
				}
			})
		}
	}
}

func readOutput(t *testing.T, path, format string) []byte {
	t.Helper()
	if format != "docx" {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = rc.Close() }()
			data, _ := io.ReadAll(rc)
			return data
		}
	}
	t.Fatalf("%s has no word/document.xml", path)
	return nil
}
