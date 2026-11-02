// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package licenses

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestTexts_MatchTheRepository: texts/ es copia fiel de LICENSE, NOTICE y
// los textos de licencia de core/renderer/chromium/third_party. Si falla,
// correr scripts/sync-licenses.sh.
func TestTexts_MatchTheRepository(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	want := map[string]string{
		"LICENSE": filepath.Join(root, "LICENSE"),
		"NOTICE":  filepath.Join(root, "NOTICE"),
	}
	src := filepath.Join(root, "core", "renderer", "chromium", "third_party")
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		base := d.Name()
		if strings.HasPrefix(base, "LICENSE") || base == "FONT-LICENSE" || base == "GUST-FONT-LICENSE.txt" || base == "LPPL-1.3c.txt" || base == "README" {
			rel, _ := filepath.Rel(src, p)
			want["third_party/"+filepath.ToSlash(rel)] = p
		}
		return nil
	})
	if err != nil {
		t.Skipf("repository layout not available: %v", err)
	}
	got := Files()
	if len(got) != len(want) {
		t.Errorf("embedded %d files, repository has %d; run scripts/sync-licenses.sh", len(got), len(want))
	}
	for name, path := range want {
		orig, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		copied, err := texts.ReadFile("texts/" + name)
		if err != nil || !bytes.Equal(orig, copied) {
			t.Errorf("%s is missing or differs from %s; run scripts/sync-licenses.sh", name, path)
		}
	}
}

func TestText_CarriesTheFontLicenses(t *testing.T) {
	text := Text()
	for _, want := range []string{
		"==== NOTICE ====",
		"Antonis Tsolomitis",
		"GUST Font License",
		"LPPL Version 1.3c",
		"newcomputermodern",
		"==== third_party/mermaid/LICENSE ====",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("licenses text is missing %q", want)
		}
	}
}
