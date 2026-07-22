package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDocLangE2E(t *testing.T) {
	tempDir := t.TempDir()
	docFile := filepath.Join(tempDir, "test.doclang")

	content := `---
title: Test
---
# Header
Test content
`
	if err := os.WriteFile(docFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Change working directory to tempDir
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get wd: %v", err)
	}
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)

	// Suppress os.Exit in Execute
	os.Args = []string{"doclang", "build", docFile, "--format", "html", "--output", "test.html"}
	cmd := NewRootCommand(Options{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("CLI Execute failed: %v", err)
	}

	// Check if output file was created
	outHTML := filepath.Join(tempDir, "test.html")
	if _, err := os.Stat(outHTML); os.IsNotExist(err) {
		t.Errorf("Expected output file %s to be created", outHTML)
	}
}
