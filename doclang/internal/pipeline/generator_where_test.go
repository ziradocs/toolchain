// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// genWarnLogger guarda los avisos del generador.
type genWarnLogger struct {
	discardLogger
	mu    sync.Mutex
	warns []string
}

func (l *genWarnLogger) Warn(msg string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warns = append(l.warns, strings.TrimSpace(fmt.Sprintf(msg, args...)))
}

// Los avisos del generador nombran la línea del archivo que escribió el
// elemento, no la del contenido expandido: con un @include el número sería
// otro. Es la misma ubicación que el reporte de --fail-on-unavailable.
func TestGenerate_WarningsNameTheSourceFileLine(t *testing.T) {
	head := "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\n    root first\n@include part.doclang\n"
	for _, tc := range []struct {
		name, format, part, want string
	}{
		{"markdown", "markdown", "  TEXT\n    filler\n  @center\n", "(part.doclang:3)"},
		{"docx", "docx", "  TEXT\n    filler\n  IMAGE \"missing.png\" \"Alt\"\n", "part.doclang:3: image"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "part.doclang"), []byte(tc.part), 0o644); err != nil {
				t.Fatal(err)
			}
			log := &genWarnLogger{}
			prepared, err := Prepare(log, []byte(head), PrepareOptions{MaxInputBytes: 1 << 20, IncludeRoot: dir, DocDir: dir})
			if err != nil {
				t.Fatalf("Prepare: %v", err)
			}
			err = Generate(log, prepared, "root.doclang", GenerateOptions{Format: tc.format, OutputFile: filepath.Join(dir, "out."+tc.format), RenderMode: "browser", DiagramBackend: "chromium", AssetRoot: dir, SourceDir: dir})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			joined := strings.Join(log.warns, "\n")
			if !strings.Contains(joined, tc.want) {
				t.Errorf("want a warning with %q, got:\n%s", tc.want, joined)
			}
		})
	}
}

type infoLogger struct {
	discardLogger
	infos []string
}

func (l *infoLogger) Info(category, msg string, args ...interface{}) {
	l.infos = append(l.infos, category+": "+fmt.Sprintf(msg, args...))
}

// El Info que señala el primer uso de un nodeId repetido sale en la consola,
// con su archivo y línea, para que el error tenga a dónde apuntar.
func TestPrepare_ShowsTheNoteThatPointsAtTheFirstNodeID(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "part.doclang"), []byte("  <!-- node-id: Dup -->\n  TEXT\n    uno\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := "---\nmode: strict\ntitle: \"D\"\n---\n\nSECTION \"A\"\n  TEXT\n    antes\n@include part.doclang\n  <!-- node-id: Dup -->\n  TEXT\n    dos\n"
	log := &infoLogger{}
	_, err := Prepare(log, []byte(root), PrepareOptions{MaxInputBytes: 1 << 20, IncludeRoot: dir, DocDir: dir})
	if err == nil {
		t.Fatal("a repeated nodeId must fail the build")
	}
	if got := strings.Join(log.infos, "\n"); !strings.Contains(got, `part.doclang:2`) || !strings.Contains(got, `nodeId "Dup" is first used here`) {
		t.Errorf("the note is not shown: %q", got)
	}
}
