package transform

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/diagnostics"
)

func tableFilterFixture() *ast.AST {
	pos := diagnostics.NewPosition(1, 1)
	doc := ast.NewAST(pos)
	block := ast.NewContentBlock(pos, "content")
	table := ast.NewTableElement(pos)
	table.NodeID = "TableA"
	table.TableRows = []ast.TableRow{{NodeID: "RowA", Cells: []ast.TableRowCell{{NodeID: "CellA", Content: "A"}}}}
	table.SyncTableViews()
	block.Elements = append(block.Elements, table)
	doc.ContentBlocks = append(doc.ContentBlocks, *block)
	ast.SetTableContract(doc)
	return doc
}

func writeFilter(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "filter.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

const compatibleHandshake = `if [ "$1" = "--ziradocs-capabilities" ]; then
  echo '{"astSchemaVersions":["2.15.0"],"features":["table-rows-v1"]}'
  exit 0
fi
`

func TestTableRowsFilterHandshakeAndPreservation(t *testing.T) {
	good := writeFilter(t, compatibleHandshake+"cat")
	doc, err := RunFilters(tableFilterFixture(), []string{good}, time.Second*2)
	if err != nil {
		t.Fatal(err)
	}
	if target, ok := ast.ResolveTableIdentity(doc, "CellA"); !ok || target.Kind != "cell" {
		t.Fatal("filter lost identity")
	}
	changed := writeFilter(t, compatibleHandshake+`sed 's/RowA/RowChanged/g'`)
	if _, err := RunFilters(tableFilterFixture(), []string{changed}, time.Second*2); err == nil || !strings.Contains(err.Error(), "identities") {
		t.Fatalf("renamed row ID accepted: %v", err)
	}
	removedCellID := writeFilter(t, compatibleHandshake+`sed 's/"nodeId":"CellA"/"nodeId":""/g'`)
	if _, err := RunFilters(tableFilterFixture(), []string{removedCellID}, time.Second*2); err == nil || !strings.Contains(err.Error(), "identities") {
		t.Fatalf("removed cell ID accepted: %v", err)
	}
	downgrade := writeFilter(t, compatibleHandshake+`sed 's/2\.15\.0/2.14.0/g'`)
	if _, err := RunFilters(tableFilterFixture(), []string{downgrade}, time.Second*2); err == nil || !strings.Contains(err.Error(), "legacy schemaVersion") {
		t.Fatalf("downgrade accepted: %v", err)
	}
	legacyBytes, err := json.Marshal(ast.NewAST(diagnostics.NewPosition(1, 1)))
	if err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(t.TempDir(), "legacy.json")
	if err := os.WriteFile(legacyPath, legacyBytes, 0600); err != nil {
		t.Fatal(err)
	}
	lost := writeFilter(t, compatibleHandshake+"cat "+legacyPath)
	if _, err := RunFilters(tableFilterFixture(), []string{lost}, time.Second*2); err == nil || !strings.Contains(err.Error(), "identities") {
		t.Fatalf("filter removed tableRows without rejection: %v", err)
	}
}

func TestTableRowsFilterIncompatibleNeverReceivesAST(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "invoked")
	bad := writeFilter(t, "if [ \"$1\" = \"--ziradocs-capabilities\" ]; then echo help; exit 0; fi\necho ran >"+marker+"\ncat")
	if _, err := RunFilters(tableFilterFixture(), []string{bad}, time.Second*2); err == nil || !strings.Contains(err.Error(), "capability") {
		t.Fatalf("incompatible filter accepted: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("AST was sent after failed handshake: %v", err)
	}
	good := writeFilter(t, compatibleHandshake+"cat")
	if _, err := RunFilters(tableFilterFixture(), []string{good, bad}, time.Second*2); err == nil {
		t.Fatal("incompatible second filter in chain accepted")
	}
	missingFeature := writeFilter(t, `if [ "$1" = "--ziradocs-capabilities" ]; then echo '{"astSchemaVersions":["2.15.0"],"features":[]}'; exit 0; fi
cat`)
	if _, err := RunFilters(tableFilterFixture(), []string{missingFeature}, time.Second*2); err == nil || !strings.Contains(err.Error(), "does not support") {
		t.Fatalf("missing feature accepted: %v", err)
	}
	oversize := writeFilter(t, `if [ "$1" = "--ziradocs-capabilities" ]; then echo '{"astSchemaVersions":["2.15.0"],"features":["table-rows-v1"]}'; printf '%5000s' ''; exit 0; fi
cat`)
	if _, err := RunFilters(tableFilterFixture(), []string{oversize}, time.Second*2); err == nil {
		t.Fatalf("oversized response accepted: %v", err)
	}
	hung := writeFilter(t, `if [ "$1" = "--ziradocs-capabilities" ]; then while :; do :; done; fi
cat`)
	if _, err := RunFilters(tableFilterFixture(), []string{hung}, time.Millisecond*40); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("hanging handshake accepted: %v", err)
	}
	// Legacy documents retain the old protocol, with no handshake.
	legacy := ast.NewAST(diagnostics.NewPosition(1, 1))
	legacyFilter := writeFilter(t, "if [ \"$1\" = \"--ziradocs-capabilities\" ]; then exit 1; fi\ncat")
	if _, err := RunFilters(legacy, []string{legacyFilter}, time.Second*2); err != nil {
		t.Fatalf("legacy filter required handshake: %v", err)
	}
}

func TestLimitedCapabilityWriter(t *testing.T) {
	w := &limitedWriter{limit: 4}
	if _, err := w.Write([]byte("abcd")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("e")); err == nil || !w.exceeded {
		t.Fatalf("oversize write accepted: %v", err)
	}
}

func TestTableRowsFilterStopsDescendantsHoldingPipes(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{"handshake timeout", `if [ "$1" = "--ziradocs-capabilities" ]; then
  (sleep 0.3; echo orphan > MARKER) &
  wait
fi
cat`, "timed out"},
		{"handshake oversize", `if [ "$1" = "--ziradocs-capabilities" ]; then
  (sleep 0.3; echo orphan > MARKER) &
  printf '%5000s' ''
  wait
fi
cat`, "exceeds"},
		{"handshake parent exits", `if [ "$1" = "--ziradocs-capabilities" ]; then
  echo '{"astSchemaVersions":["2.15.0"],"features":["table-rows-v1"]}'
  (sleep 0.3; echo orphan > MARKER) &
  exit 0
fi
cat`, "handshake failed"},
		{"filter timeout", compatibleHandshake + `(sleep 0.3; echo orphan > MARKER) &
wait`, "timed out"},
		{"filter parent exits", compatibleHandshake + `(sleep 0.3; echo orphan > MARKER) &
exit 0`, "exited with error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "orphan")
			path := writeFilter(t, strings.ReplaceAll(tc.body, "MARKER", marker))
			start := time.Now()
			_, err := RunFilters(tableFilterFixture(), []string{path}, 50*time.Millisecond)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("unexpected result: %v", err)
			}
			if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
				t.Fatalf("filter exceeded bounded wait: %s", elapsed)
			}
			time.Sleep(350 * time.Millisecond)
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("descendant survived cancellation: %v", err)
			}
		})
	}
}
