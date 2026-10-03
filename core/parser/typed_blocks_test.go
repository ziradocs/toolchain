// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package parser

import (
	"encoding/json"
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/util"
)

func firstBlock(t *testing.T, src string) (*ast.SpecialBlockElement, []string) {
	t.Helper()
	doc, diags := New(util.NewNoop()).Parse(src, "t.slidelang")
	var rules []string
	for _, d := range diags {
		if d.IsError() {
			t.Fatalf("unexpected error: %s", d.String())
		}
		rules = append(rules, d.RuleID)
	}
	for _, b := range doc.ContentBlocks {
		for _, el := range b.Elements {
			if sb, ok := el.(*ast.SpecialBlockElement); ok {
				return sb, rules
			}
		}
	}
	t.Fatal("no special block in the document")
	return nil, nil
}

func elementsJSON(t *testing.T, els []ast.Element) string {
	t.Helper()
	raw, err := json.Marshal(els)
	if err != nil {
		t.Fatal(err)
	}
	var tree interface{}
	_ = json.Unmarshal(raw, &tree)
	return dropPositions(tree)
}

func dropPositions(v interface{}) string {
	var strip func(interface{}) interface{}
	strip = func(v interface{}) interface{} {
		switch x := v.(type) {
		case map[string]interface{}:
			out := map[string]interface{}{}
			for k, val := range x {
				if k == "position" || k == "endPosition" {
					continue
				}
				out[k] = strip(val)
			}
			return out
		case []interface{}:
			out := make([]interface{}, len(x))
			for i, val := range x {
				out[i] = strip(val)
			}
			return out
		}
		return v
	}
	b, _ := json.Marshal(strip(v))
	return string(b)
}

const (
	strictHead = "---\nmode: strict\n---\n\nSLIDE content\n  title: \"T\"\n"
	flexHead   = "---\nmode: flex\n---\n\n## T\n\n"
)

// A `typed` attribute on a strict block's opening line reads the body the way a
// flex block is read: the AST equals the one the flex source produces.
func TestTypedBlock_StrictReadsTheBodyLikeFlex(t *testing.T) {
	body := "### Improved performance\nFaster builds.\n- one\n- two\n"
	flexBlock, _ := firstBlock(t, flexHead+":::card\n"+body+":::\n")
	if len(flexBlock.Elements) == 0 {
		t.Fatal("the flex block should have nested elements")
	}

	typed, rules := firstBlock(t, strictHead+"  :::card{typed}\n"+indentLines(body, "  ")+"  :::\n")
	if len(rules) != 0 {
		t.Errorf("unexpected diagnostics %v", rules)
	}
	if typed.BlockType != "card" || typed.Title != "" {
		t.Errorf("type/title = %q/%q, want card/empty", typed.BlockType, typed.Title)
	}
	if typed.Content != flexBlock.Content {
		t.Errorf("Content = %q, want %q", typed.Content, flexBlock.Content)
	}
	if got, want := elementsJSON(t, typed.Elements), elementsJSON(t, flexBlock.Elements); got != want {
		t.Errorf("Elements differ from flex\n strict: %s\n flex:   %s", got, want)
	}
}

func indentLines(s, pad string) string {
	var b strings.Builder
	for _, l := range strings.SplitAfter(s, "\n") {
		if l == "" {
			continue
		}
		b.WriteString(pad + l)
	}
	return b.String()
}

// Without the attribute a strict block reads exactly as before.
func TestTypedBlock_WithoutTheMarkerNothingChanges(t *testing.T) {
	sb, rules := firstBlock(t, strictHead+"  :::card\n  ### Heading\n  body\n  :::\n")
	if len(sb.Elements) != 0 {
		t.Errorf("a strict block without the marker has no nested heading, got %d elements", len(sb.Elements))
	}
	if len(rules) != 0 {
		t.Errorf("unexpected diagnostics %v", rules)
	}
}

// The marker is an attribute, so a title never meets it: a title that ends in the
// word "typed" is a title, with or without nested elements.
func TestTypedBlock_ATitleEndingInTypedIsStillATitle(t *testing.T) {
	sb, rules := firstBlock(t, strictHead+"  :::info Dynamically typed\n  ### Heading\n  body\n  :::\n")
	if sb.Title != "Dynamically typed" || sb.BlockType != "info" {
		t.Errorf("type/title = %q/%q", sb.BlockType, sb.Title)
	}
	if len(sb.Elements) != 0 || len(rules) != 0 {
		t.Errorf("a block without the attribute must read as before: %d elements, %v", len(sb.Elements), rules)
	}
}

// A titled block carries the marker in the attribute list, next to any others.
func TestTypedBlock_TitleAndOtherAttributes(t *testing.T) {
	sb, _ := firstBlock(t, strictHead+"  :::details{typed} Advanced settings\n  ### Heading\n  body\n  :::\n")
	if sb.BlockType != "details" || sb.Title != "Advanced settings" || len(sb.Elements) == 0 {
		t.Errorf("type/title = %q/%q, %d elements", sb.BlockType, sb.Title, len(sb.Elements))
	}

	flex, _ := firstBlock(t, flexHead+":::card{type=\"success\"}\n### Heading\nbody\n:::\n")
	strict, _ := firstBlock(t, strictHead+"  :::card{type=\"success\" typed}\n  ### Heading\n  body\n  :::\n")
	if strict.BlockType != flex.BlockType {
		t.Errorf("BlockType = %q, want the flex %q", strict.BlockType, flex.BlockType)
	}
	if got, want := elementsJSON(t, strict.Elements), elementsJSON(t, flex.Elements); got != want {
		t.Errorf("Elements differ from flex\n strict: %s\n flex:   %s", got, want)
	}
}

// Only the whole word outside quotes is the flag.
func TestTypedBlock_TypedInsideAQuotedValueIsNotTheFlag(t *testing.T) {
	sb, _ := firstBlock(t, strictHead+"  :::card{label=\"a typed b\"}\n  ### Heading\n  body\n  :::\n")
	// The block type is read up to the first space, as it always was, so the
	// quoted word ends up in the title; what matters is that it is not the flag.
	if len(sb.Elements) != 0 || !strings.Contains(sb.BlockType+" "+sb.Title, "typed") {
		t.Errorf("a quoted typed is part of the attribute value: %q %q, %d elements", sb.BlockType, sb.Title, len(sb.Elements))
	}
}

// Flex already reads every block that way, so the attribute there is just text of
// the block type, like any other attribute.
func TestTypedBlock_FlexKeepsTheAttributeInTheType(t *testing.T) {
	sb, rules := firstBlock(t, flexHead+":::card{typed}\n### Heading\nbody\n:::\n")
	if sb.BlockType != "card{typed}" {
		t.Errorf("BlockType = %q, want card{typed}", sb.BlockType)
	}
	if len(rules) != 0 {
		t.Errorf("unexpected diagnostics %v", rules)
	}
}
