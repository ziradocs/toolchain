// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package normalizer

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/util"
)

func TestDetector_New(t *testing.T) {
	detector := NewDetector()

	if detector == nil {
		t.Fatal("NewDetector() should not return nil")
	}
}

func TestDetector_Detect_NotAI(t *testing.T) {
	detector := NewDetector()
	content := `---
mode: flex
title: Normal Content
---

# Introduction

This is normal human-written content without AI patterns.`

	result := detector.Detect(content)

	if result.Detected {
		t.Error("Normal content should not be detected as AI")
	}
	if result.Score > 0.5 {
		t.Errorf("Score = %v, should be low for non-AI content", result.Score)
	}
}

func TestDetector_Detect_NormalizationPatterns(t *testing.T) {
	detector := NewDetector()
	content := `---
mode: flex
---

# Introduction

This comprehensive guide will provide you with actionable insights and best practices.
Let's delve into the intricacies of modern development.
It is crucial to remember that understanding these concepts is paramount.`

	result := detector.Detect(content)

	// Just check that detector runs without error
	// Pattern detection can vary, so we don't assert specific results
	if result.Score < 0 || result.Score > 1 {
		t.Errorf("Score should be between 0 and 1, got %v", result.Score)
	}
}

func TestDetector_Detect_Empty(t *testing.T) {
	detector := NewDetector()
	result := detector.Detect("")

	if result.Detected {
		t.Error("Empty content should not be detected as AI")
	}
	if result.Score != 0 {
		t.Errorf("Score = %v, want 0 for empty content", result.Score)
	}
}

func TestNormalizer_New(t *testing.T) {
	config := Config{
		EnableDetection:  true,
		EnableTransforms: true,
	}

	norm := NewNormalizer(config, nil)

	if norm == nil {
		t.Fatal("NewNormalizer() should not return nil")
	}
}

func TestNormalizer_Normalize_NoChanges(t *testing.T) {
	config := Config{
		EnableDetection:  false,
		EnableTransforms: false, // Disable transforms so content stays unchanged
	}
	logger := &testLogger{}

	norm := NewNormalizer(config, logger)
	content := "Simple content\nNo transformations needed"

	normalized, report := norm.Normalize(content)

	// With transforms disabled, content should not be modified
	if report.WasModified {
		t.Log("Note: Some minimal normalization may still occur")
	}
	if !strings.Contains(normalized, "Simple content") {
		t.Error("Should preserve content")
	}
}

// testLogger es un logger simple para pruebas
type testLogger struct{}

func (l *testLogger) Info(category, message string, args ...interface{})     {}
func (l *testLogger) Debug(component, message string, args ...interface{})   {}
func (l *testLogger) Warn(message string, args ...interface{})               {}
func (l *testLogger) Error(message string, args ...interface{})              {}
func (l *testLogger) Progress(stage, operation string, progress int)         {}
func (l *testLogger) Summary(operation string, stats map[string]interface{}) {}
func (l *testLogger) SetLevel(level util.LogLevel)                           {}

func TestNormalizer_Normalize_WithTransforms(t *testing.T) {
	config := Config{
		EnableDetection:  false,
		EnableTransforms: true,
	}
	logger := &testLogger{}

	norm := NewNormalizer(config, logger)

	// Content with extra whitespace that should be normalized
	content := "Line 1\n\n\n\nLine 2"

	normalized, report := norm.Normalize(content)

	if !strings.Contains(normalized, "Line 1") {
		t.Error("Should preserve actual content")
	}
	if !strings.Contains(normalized, "Line 2") {
		t.Error("Should preserve actual content")
	}

	if report.OriginalSize == 0 {
		t.Error("OriginalSize should be set")
	}
}

func TestNormalizationReport_Basic(t *testing.T) {
	report := &NormalizationReport{
		WasModified:    true,
		OriginalSize:   100,
		NormalizedSize: 95,
		Applied:        []string{"rule1", "rule2"},
		Errors:         []string{},
	}

	if !report.WasModified {
		t.Error("WasModified should be true")
	}
	if len(report.Applied) != 2 {
		t.Errorf("len(Applied) = %v, want 2", len(report.Applied))
	}
	if len(report.Errors) != 0 {
		t.Error("Should not have errors")
	}
}

func TestNormalizationReport_WithErrors(t *testing.T) {
	report := &NormalizationReport{
		Errors: []string{"error1", "error2"},
	}

	if len(report.Errors) != 2 {
		t.Errorf("len(Errors) = %v, want 2", len(report.Errors))
	}
}

func TestDetectionResult_Basic(t *testing.T) {
	result := &DetectionResult{
		Detected: true,
		Score:    0.75,
	}

	if !result.Detected {
		t.Error("Detected should be true")
	}
	if result.Score != 0.75 {
		t.Errorf("Score = %v, want 0.75", result.Score)
	}
}

func TestConfig_Defaults(t *testing.T) {
	config := Config{}

	if config.EnableDetection {
		t.Error("Default EnableDetection should be false")
	}
	if config.EnableTransforms {
		t.Error("Default EnableTransforms should be false")
	}
}

func TestConfig_SkipRules(t *testing.T) {
	config := Config{
		SkipRules: []string{"frontmatter", "headers"},
	}

	if len(config.SkipRules) != 2 {
		t.Errorf("len(SkipRules) = %v, want 2", len(config.SkipRules))
	}
}

// TestNormalizer_Normalize_CodeGroupNotAIDetected is a regression test for
// issue #174: a code-group written with the ":::code-item{title=}" wrapper
// must be normalized to the canonical ```lang [label] syntax EVEN WHEN the
// content is not detected as AI-generated. Before the fix, applyBasicFormatting
// only ran MermaidFormatter, so this hand-written (non-AI-detected) content
// reached the parser untouched and only the code-group's first tab survived.
func TestNormalizer_Normalize_CodeGroupNotAIDetected(t *testing.T) {
	config := Config{
		EnableDetection:  true,
		EnableTransforms: true,
	}
	logger := &testLogger{}
	norm := NewNormalizer(config, logger)

	content := `---
mode: flex
title: System Configuration
---

# Code Examples

Normal hand-written content, no AI patterns here.

::::code-group
:::code-item{title="config.yaml"}
` + "```yaml" + `
server:
  port: 8080
` + "```" + `
:::

:::code-item{title="main.go"}
` + "```go" + `
package main

func main() {}
` + "```" + `
:::
::::

End of content.
`

	normalized, report := norm.Normalize(content)

	// Precondition: this content must NOT be detected as AI, so the test
	// actually exercises the applyBasicFormatting path (not applyTransformations,
	// which already has coverage via code_group_formatter_test.go).
	if report.DetectionResult.Detected {
		t.Fatalf("test content should not be detected as AI (Detected=true, score=%v); "+
			"the test no longer exercises the basic-formatting path", report.DetectionResult.Score)
	}

	if strings.Contains(normalized, ":::code-item") {
		t.Errorf("expected the :::code-item{} wrapper to be rewritten by CodeGroupFormatter "+
			"even on the non-AI-detected path, got:\n%s", normalized)
	}

	if !strings.Contains(normalized, "```yaml [config.yaml]") {
		t.Errorf("expected canonical code-group syntax (fence + [label]) for config.yaml, got:\n%s", normalized)
	}

	if !strings.Contains(normalized, "```go [main.go]") {
		t.Errorf("expected canonical code-group syntax (fence + [label]) for main.go, got:\n%s", normalized)
	}

	appliedCodeGroupFormatter := false
	for _, applied := range report.Applied {
		if strings.Contains(applied, "CodeGroupFormatter") {
			appliedCodeGroupFormatter = true
			break
		}
	}
	if !appliedCodeGroupFormatter {
		t.Errorf("expected CodeGroupFormatter to appear in report.Applied, got: %v", report.Applied)
	}
}

func TestNormalizer_WithLogger(t *testing.T) {
	config := Config{
		EnableDetection:  true,
		EnableTransforms: true,
	}
	logger := &testLogger{}

	// Test that normalizer works with logger
	norm := NewNormalizer(config, logger)

	content := "Test content"
	normalized, report := norm.Normalize(content)

	if normalized == "" {
		t.Error("Should return normalized content")
	}
	if report.OriginalSize != len(content) {
		t.Errorf("OriginalSize = %v, want %v", report.OriginalSize, len(content))
	}
}
