// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package fragment

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
	"go.ziradocs.com/core/v2/formatter"
	"go.ziradocs.com/core/v2/parser"
	"go.ziradocs.com/core/v2/util"
)

type FragmentNode struct {
	Path              string               `json:"path"`
	Type              string               `json:"type"`
	NodeID            string               `json:"nodeId,omitempty"`
	Position          diagnostics.Position `json:"position"`
	EffectivePosition diagnostics.Position `json:"effectivePosition"`
	Anchor            string               `json:"anchor,omitempty"`
	PayloadSHA256     string               `json:"payloadSHA256"`
}
type ContentFragmentReceipt struct {
	Version               string                   `json:"version"`
	Accepted              bool                     `json:"accepted"`
	OutputProduced        bool                     `json:"outputProduced"`
	Namespace             string                   `json:"namespace"`
	OriginalMarkdown      string                   `json:"originalMarkdown"`
	SourceSHA256          string                   `json:"sourceSHA256"`
	EffectiveSourceSHA256 string                   `json:"effectiveSourceSHA256,omitempty"`
	SchemaVersion         string                   `json:"schemaVersion"`
	SourceCapabilities    []string                 `json:"sourceCapabilities"`
	Capabilities          []string                 `json:"capabilities"`
	Nodes                 []FragmentNode           `json:"nodes"`
	Diagnostics           []diagnostics.Diagnostic `json:"diagnostics"`
}
type ContentFragmentResult struct {
	Source   string
	Elements []ast.Element
	Receipt  ContentFragmentReceipt
}

// NormalizeMarkdownContent never mutates the original. Native strict reparse
// must conserve every payload and identity before any output is published.
func NormalizeMarkdownContent(original string, options parser.ContentOptions) ContentFragmentResult {
	r := ContentFragmentResult{Receipt: ContentFragmentReceipt{Version: "content-markdown-normalization-v1", Namespace: options.AnchorNamespace, OriginalMarkdown: original, SourceSHA256: fragmentHash([]byte(original)), Nodes: []FragmentNode{}, Diagnostics: []diagnostics.Diagnostic{}}}
	nodes, diags := parser.ParseMarkdownContent(original, options)
	if len(diags) > 0 {
		r.Receipt.Diagnostics = diags
	}
	reject := func(message string) ContentFragmentResult {
		r.Receipt.Diagnostics = append(r.Receipt.Diagnostics, diagnostics.NewError(message, diagnostics.NewPosition(1, 1), "content-markdown"))
		return r
	}
	for _, d := range diags {
		if d.IsError() {
			return r
		}
	}
	if len(nodes) == 0 {
		return reject("empty content fragment")
	}
	// IDs are path-based within stable namespace; global source offsets are not identity.
	doc := fragmentDocument(nodes)
	index := 0
	_ = ast.Walk(doc, func(n ast.Node) error {
		if _, ok := n.(ast.Element); ok || n.GetType() == ast.NodeTypePointItem || n.GetType() == ast.NodeTypeChecklistItem {
			if id, ok := n.(interface{ SetNodeID(string) }); ok {
				index++
				id.SetNodeID("e" + fragmentHash([]byte(fmt.Sprintf("%s|%d", options.AnchorNamespace, index)))[:26])
			}
		}
		return nil
	})
	r.Receipt.Capabilities = formatter.ContentFragmentCapabilities(nodes)
	if r.Receipt.Capabilities == nil {
		r.Receipt.Capabilities = []string{}
	}
	ast.SetTableContract(doc)
	r.Receipt.SchemaVersion = doc.SchemaVersion
	source, err := formatter.FormatContentFragment(nodes)
	if err != nil {
		return reject(err.Error())
	}
	r.Receipt.SourceCapabilities = []string{}
	for _, c := range r.Receipt.Capabilities {
		if c == ast.TypedHeadingsCapability || c == ast.ContentMarkdownCapability || c == ast.NestedListTypesCapability || c == ast.ListStartCapability || c == ast.MathSourceCapability {
			r.Receipt.SourceCapabilities = append(r.Receipt.SourceCapabilities, c)
		}
	}
	wrapper := "---\nmode: strict\n"
	if len(r.Receipt.SourceCapabilities) > 0 {
		wrapper += "ast_capabilities:\n"
		for _, c := range r.Receipt.SourceCapabilities {
			wrapper += "  - " + c + "\n"
		}
	}
	prefix := wrapper + "---\nSLIDE content\n"
	wrapper = prefix + source
	p := parser.New(util.NewNoop())
	effective, diags := p.Parse(wrapper, "")
	for _, d := range diags {
		if d.IsError() {
			return reject("strict fragment reparse: " + d.Message)
		}
	}
	if effective == nil || len(effective.ContentBlocks) != 1 || !reflect.DeepEqual(fragmentPayload(nodes), fragmentPayload(effective.ContentBlocks[0].Elements)) {
		return reject("strict fragment cannot preserve the original parsed payload and identities")
	}
	r.Receipt.Nodes = fragmentNodes(nodes)
	afterNodes := fragmentNodes(effective.ContentBlocks[0].Elements)
	for i := range r.Receipt.Nodes {
		for _, after := range afterNodes {
			if after.NodeID == r.Receipt.Nodes[i].NodeID {
				r.Receipt.Nodes[i].EffectivePosition = after.Position
				r.Receipt.Nodes[i].EffectivePosition.Line -= strings.Count(prefix, "\n")
			}
		}
	}
	r.Receipt.Accepted = true
	r.Receipt.EffectiveSourceSHA256 = fragmentHash([]byte(source))
	r.Source = source
	r.Elements = nodes
	return r
}
func fragmentDocument(nodes []ast.Element) *ast.AST {
	doc := ast.NewAST(diagnostics.NewPosition(1, 1))
	b := ast.NewContentBlock(diagnostics.NewPosition(1, 1), "content")
	b.Elements = nodes
	doc.ContentBlocks = []ast.ContentBlock{*b}
	doc.FrontMatter = &ast.FrontMatterNode{Mode: "strict", ASTCapabilities: []string{ast.TypedHeadingsCapability, ast.ContentMarkdownCapability}}
	return doc
}
func fragmentHash(bytes []byte) string { return fmt.Sprintf("%x", sha256.Sum256(bytes)) }
func fragmentPayload(value any) any {
	bytes, _ := json.Marshal(value)
	var decoded any
	_ = json.Unmarshal(bytes, &decoded)
	var clean func(any) any
	clean = func(value any) any {
		switch v := value.(type) {
		case []any:
			for i := range v {
				v[i] = clean(v[i])
			}
		case map[string]any:
			if _, node := v["type"]; node {
				delete(v, "position")
				delete(v, "endPosition")
				if v["type"] == "table" {
					delete(v, "rowPositions")
				}
			}
			for k, x := range v {
				v[k] = clean(x)
			}
		}
		return value
	}
	return clean(decoded)
}
func fragmentNodes(nodes []ast.Element) []FragmentNode {
	bytes, _ := json.Marshal(nodes)
	var value any
	_ = json.Unmarshal(bytes, &value)
	out := []FragmentNode{}
	var walk func(any, string)
	walk = func(value any, path string) {
		switch v := value.(type) {
		case []any:
			for i, x := range v {
				walk(x, fmt.Sprintf("%s/%d", path, i))
			}
		case map[string]any:
			if kind, ok := v["type"].(string); ok {
				var pos diagnostics.Position
				b, _ := json.Marshal(v["position"])
				_ = json.Unmarshal(b, &pos)
				id, _ := v["nodeId"].(string)
				b, _ = json.Marshal(fragmentPayload(v))
				anchor, _ := v["anchor"].(string)
				out = append(out, FragmentNode{Path: path, Type: kind, NodeID: id, Position: pos, Anchor: anchor, PayloadSHA256: fragmentHash(b)})
			}
			for _, k := range []string{"elements", "items", "subPoints", "subItems", "columns", "codeBlocks"} {
				if x, ok := v[k]; ok {
					walk(x, path+"/"+k)
				}
			}
		}
	}
	walk(value, "/elements")
	return out
}

// PublishContentFragment refuses existing paths. A rejected conversion publishes
// only a receipt, and cannot truncate a previously retained source.
func PublishContentFragment(output, receiptPath string, result ContentFragmentResult) error {
	if result.Receipt.SourceSHA256 != fragmentHash([]byte(result.Receipt.OriginalMarkdown)) || result.Receipt.OutputProduced || result.Receipt.Accepted && result.Receipt.EffectiveSourceSHA256 != fragmentHash([]byte(result.Source)) {
		return fmt.Errorf("fragment receipt/source binding is invalid")
	}
	if output == "" || receiptPath == "" {
		return fmt.Errorf("fragment output and receipt paths are required")
	}
	a, _ := filepath.Abs(output)
	b, _ := filepath.Abs(receiptPath)
	if a == b {
		return fmt.Errorf("fragment output and receipt paths must differ")
	}
	for _, name := range []string{output, receiptPath} {
		if _, err := os.Lstat(name); err == nil {
			return fmt.Errorf("fragment destination exists: %s", name)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	staged := []string{}
	defer func() {
		for _, name := range staged {
			_ = os.Remove(name)
		}
	}()
	stage := func(destination string, bytes []byte) (string, error) {
		f, err := os.CreateTemp(filepath.Dir(destination), ".content-fragment-*")
		if err != nil {
			return "", err
		}
		staged = append(staged, f.Name())
		if _, err = f.Write(bytes); err != nil {
			_ = f.Close()
			return "", err
		}
		if err = f.Close(); err != nil {
			return "", err
		}
		return f.Name(), nil
	}
	receipt := result.Receipt
	receipt.OutputProduced = receipt.Accepted
	bytes, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return err
	}
	bytes = append(bytes, '\n')
	side, err := stage(receiptPath, bytes)
	if err != nil {
		return err
	}
	if !receipt.Accepted {
		return os.Link(side, receiptPath)
	}
	source, err := stage(output, []byte(result.Source))
	if err != nil {
		return err
	}
	if err = os.Link(side, receiptPath); err != nil {
		return err
	}
	if err = os.Link(source, output); err != nil {
		if cleanup := os.Remove(receiptPath); cleanup != nil {
			return fmt.Errorf("publication failed (%w); receipt rollback failed: %v", err, cleanup)
		}
		return err
	}
	return nil
}
