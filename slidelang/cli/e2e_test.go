package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSlideLangE2E(t *testing.T) {
	tempDir := t.TempDir()
	slideFile := filepath.Join(tempDir, "test.slidelang")

	content := `---
title: Test Slide
---
# Welcome
To the test
`
	if err := os.WriteFile(slideFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get wd: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to chdir: %v", err)
	}

	os.Args = []string{"slidelang", "build", slideFile, "--format", "html", "--output", "test.html"}
	cmd := NewRootCommand(Options{})
	
	if err := cmd.Execute(); err != nil {
		t.Fatalf("CLI Execute failed: %v", err)
	}

	outHTML := filepath.Join(tempDir, "test.html")
	if _, err := os.Stat(outHTML); os.IsNotExist(err) {
		t.Errorf("Expected output file %s to be created", outHTML)
	}
}
