// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package ast

import (
	"testing"

	"go.ziradocs.com/core/diagnostics"
)

func TestNewFrontMatterNode(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	fm := NewFrontMatterNode(pos)

	if fm == nil {
		t.Fatal("NewFrontMatterNode() returned nil")
	}

	if fm.GetType() != NodeTypeFrontMatter {
		t.Errorf("GetType() = %v, want NodeTypeFrontMatter", fm.GetType())
	}

	if fm.Variables == nil {
		t.Fatal("NewFrontMatterNode() did not initialize Variables")
	}

	if len(fm.Variables) != 0 {
		t.Errorf("Variables length = %d, want 0", len(fm.Variables))
	}
}

func TestFrontMatterNode_Fields(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	fm := NewFrontMatterNode(pos)

	fm.Mode = "presentation"
	fm.Title = "Test Presentation"
	fm.Author = "Test Author"
	fm.Date = "2024-01-01"
	fm.Theme = "default"

	if fm.Mode != "presentation" {
		t.Errorf("Mode = %s, want presentation", fm.Mode)
	}

	if fm.Title != "Test Presentation" {
		t.Errorf("Title = %s, want Test Presentation", fm.Title)
	}

	if fm.Author != "Test Author" {
		t.Errorf("Author = %s, want Test Author", fm.Author)
	}
}

func TestFrontMatterNode_Variables(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	fm := NewFrontMatterNode(pos)

	fm.Variables["color"] = "#FF0000"
	fm.Variables["size"] = 42

	if fm.Variables["color"] != "#FF0000" {
		t.Errorf("Variables[color] = %v, want #FF0000", fm.Variables["color"])
	}

	if fm.Variables["size"] != 42 {
		t.Errorf("Variables[size] = %v, want 42", fm.Variables["size"])
	}
}

func TestHeaderConfig(t *testing.T) {
	header := &HeaderConfig{
		Enabled:    true,
		Height:     "60px",
		Background: "#FFFFFF",
	}

	if !header.Enabled {
		t.Error("Enabled should be true")
	}

	if header.Height != "60px" {
		t.Errorf("Height = %s, want 60px", header.Height)
	}
}

func TestFooterConfig(t *testing.T) {
	footer := &FooterConfig{
		Enabled:    true,
		Height:     "40px",
		Background: "#F0F0F0",
	}

	if !footer.Enabled {
		t.Error("Enabled should be true")
	}

	if footer.Height != "40px" {
		t.Errorf("Height = %s, want 40px", footer.Height)
	}
}

func TestHeaderFooterText(t *testing.T) {
	text := &HeaderFooterText{
		Left:   "Left Text",
		Center: "Center Text",
		Right:  "Right Text",
	}

	if text.Left != "Left Text" {
		t.Errorf("Left = %s, want Left Text", text.Left)
	}

	if text.Center != "Center Text" {
		t.Errorf("Center = %s, want Center Text", text.Center)
	}

	if text.Right != "Right Text" {
		t.Errorf("Right = %s, want Right Text", text.Right)
	}
}

func TestPageNumbersConfig(t *testing.T) {
	pageNumbers := &PageNumbersConfig{
		Enabled:              true,
		Format:               "{{current}} / {{total}}",
		Position:             "right",
		ExcludeTitleSlides:   true,
		ExcludeClosingSlides: false,
		StartFrom:            1,
		Style:                "normal",
	}

	if !pageNumbers.Enabled {
		t.Error("Enabled should be true")
	}

	if pageNumbers.Format != "{{current}} / {{total}}" {
		t.Errorf("Format = %s, want {{current}} / {{total}}", pageNumbers.Format)
	}

	if pageNumbers.Position != "right" {
		t.Errorf("Position = %s, want right", pageNumbers.Position)
	}

	if !pageNumbers.ExcludeTitleSlides {
		t.Error("ExcludeTitleSlides should be true")
	}

	if pageNumbers.StartFrom != 1 {
		t.Errorf("StartFrom = %d, want 1", pageNumbers.StartFrom)
	}
}

func TestLogoConfig(t *testing.T) {
	logo := &LogoConfig{
		Source:   "/path/to/logo.png",
		Alt:      "Company Logo",
		Height:   "50px",
		Position: "left",
	}

	if logo.Source != "/path/to/logo.png" {
		t.Errorf("Source = %s, want /path/to/logo.png", logo.Source)
	}

	if logo.Alt != "Company Logo" {
		t.Errorf("Alt = %s, want Company Logo", logo.Alt)
	}
}

func TestBorderConfig(t *testing.T) {
	border := &BorderConfig{
		Enabled:  true,
		Color:    "#000000",
		Width:    "1px",
		Style:    "solid",
		Position: "bottom",
	}

	if !border.Enabled {
		t.Error("Enabled should be true")
	}

	if border.Color != "#000000" {
		t.Errorf("Color = %s, want #000000", border.Color)
	}

	if border.Width != "1px" {
		t.Errorf("Width = %s, want 1px", border.Width)
	}

	if border.Style != "solid" {
		t.Errorf("Style = %s, want solid", border.Style)
	}

	if border.Position != "bottom" {
		t.Errorf("Position = %s, want bottom", border.Position)
	}
}

func TestHeaderFooterConfig(t *testing.T) {
	config := &HeaderFooterConfig{
		Header: &HeaderConfig{
			Enabled: true,
			Height:  "60px",
		},
		Footer: &FooterConfig{
			Enabled: true,
			Height:  "40px",
		},
		LayoutDefaults: make(map[string]*LayoutHeaderFooterConfig),
	}

	if config.Header == nil {
		t.Fatal("Header should not be nil")
	}

	if config.Footer == nil {
		t.Fatal("Footer should not be nil")
	}

	if !config.Header.Enabled {
		t.Error("Header should be enabled")
	}

	if !config.Footer.Enabled {
		t.Error("Footer should be enabled")
	}
}

func TestLayoutHeaderFooterConfig(t *testing.T) {
	layoutConfig := &LayoutHeaderFooterConfig{
		Header: &HeaderConfig{
			Enabled: false,
		},
		Footer: &FooterConfig{
			Enabled: true,
		},
	}

	if layoutConfig.Header.Enabled {
		t.Error("Header should be disabled in layout config")
	}

	if !layoutConfig.Footer.Enabled {
		t.Error("Footer should be enabled in layout config")
	}
}

func TestContentBlockHeaderFooterOverride(t *testing.T) {
	override := &ContentBlockHeaderFooterOverride{
		Header: &HeaderConfig{
			Enabled:    false,
			Background: "#custom",
		},
	}

	if override.Header == nil {
		t.Fatal("Header should not be nil")
	}

	if override.Header.Enabled {
		t.Error("Header should be disabled in override")
	}

	if override.Header.Background != "#custom" {
		t.Errorf("Background = %s, want #custom", override.Header.Background)
	}
}

func TestFrontMatterNode_HeaderFooter(t *testing.T) {
	pos := diagnostics.NewPosition(1, 1)
	fm := NewFrontMatterNode(pos)

	fm.HeaderFooter = &HeaderFooterConfig{
		Header: &HeaderConfig{
			Enabled: true,
			Text: &HeaderFooterText{
				Center: "Test Presentation",
			},
		},
	}

	if fm.HeaderFooter == nil {
		t.Fatal("HeaderFooter should not be nil")
	}

	if fm.HeaderFooter.Header == nil {
		t.Fatal("HeaderFooter.Header should not be nil")
	}

	if !fm.HeaderFooter.Header.Enabled {
		t.Error("HeaderFooter.Header should be enabled")
	}

	if fm.HeaderFooter.Header.Text.Center != "Test Presentation" {
		t.Errorf("Header text = %s, want Test Presentation", fm.HeaderFooter.Header.Text.Center)
	}
}
