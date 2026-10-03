// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package formatter

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/ast"
)

// El nombre de archivo deja de perderse: strict lo tomaba y lo descartaba, y
// la fence flex lo mezclaba con el lenguaje. Ahora llena Filename, el
// documento pasa a 2.19.0 y fmt lo conserva en los dos DSL.
func TestCodeFilenameRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		document  bool
	}{
		{"slidelang strict", "---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n  CODE typescript renewals.ts\n    const x = 1;\n", false},
		{"slidelang flex fence", "---\nmode: flex\n---\n# Deck\n\n## S\n\n```typescript renewals.ts\nconst x = 1;\n```\n", false},
		{"doclang flex title=", "---\ntitle: D\n---\n# Doc\n\n```typescript title=\"renewals.ts\"\nconst x = 1;\n```\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := mustParse(t, tc.src, tc.document)
			code := firstOfType[*ast.CodeElement](t, doc)
			if code.Language != "typescript" || code.Filename != "renewals.ts" || strings.TrimSpace(code.Content) != "const x = 1;" {
				t.Fatalf("code = %#v", code)
			}
			if doc.SchemaVersion != ast.CodeFilenameSchemaVersion {
				t.Fatalf("version = %s", doc.SchemaVersion)
			}
			format := FormatStrict
			if tc.document {
				format = FormatDocument
			}
			out, err := format(doc)
			if err != nil {
				t.Fatal(err)
			}
			again := firstOfType[*ast.CodeElement](t, mustParse(t, out, tc.document))
			if again.Language != "typescript" || again.Filename != "renewals.ts" {
				t.Fatalf("round trip = %#v\n%s", again, out)
			}
		})
	}
}

// Una etiqueta entre corchetes o un rango entre llaves no son nombres de
// archivo: conservan el comportamiento anterior.
func TestCodeFenceBracketInfoIsNotAFilename(t *testing.T) {
	doc := mustParse(t, "---\nmode: flex\n---\n# Deck\n\n## S\n\n```python {1,3-5}\nx = 1\n```\n", false)
	code := firstOfType[*ast.CodeElement](t, doc)
	if code.Filename != "" || code.Language != "python {1,3-5}" || doc.SchemaVersion != ast.LegacySchemaVersion {
		t.Fatalf("code = %#v version=%s", code, doc.SchemaVersion)
	}
}

func TestCodeFilenameRejectsExtraTokens(t *testing.T) {
	for _, src := range []string{
		"---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n  CODE ts a.ts b.ts\n    x\n",
		"---\nmode: flex\n---\n# Deck\n\n## S\n\n```ts a.ts b.ts\nx\n```\n",
		"---\nmode: flex\n---\n# Deck\n\n## S\n\n```ts title=\"a b.ts\"\nx\n```\n",
	} {
		if _, diags := parseAny(src); !anyError(diags) {
			t.Errorf("accepted extra tokens:\n%s", src)
		}
	}
	doc := mustParse(t, "---\nmode: strict\n---\nSLIDE content\n  title: \"S\"\n  CODE ts a.ts\n    x\n", false)
	firstOfType[*ast.CodeElement](t, doc).Language = ""
	if _, err := FormatStrict(doc); err == nil || !strings.Contains(err.Error(), "language") {
		t.Fatalf("filename without language formatted: %v", err)
	}
}
