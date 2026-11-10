// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package linter

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var ruleIDLiteral = regexp.MustCompile(`^[A-Z][A-Z_]*[0-9]{3}$`)

// ruleIDsIn junta los literales con forma de ID de regla ("TABLE004") del
// código Go, sin pruebas, de los directorios dados (y sus subdirectorios).
// Los comentarios no cuentan.
func ruleIDsIn(t *testing.T, dirs ...string) map[string]bool {
	t.Helper()
	ids := map[string]bool{}
	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			ast.Inspect(f, func(n ast.Node) bool {
				if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if v, err := strconv.Unquote(lit.Value); err == nil && ruleIDLiteral.MatchString(v) {
						ids[v] = true
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return ids
}

// Un ID de regla es de un solo dueño: si el parser y el linter usan el
// mismo, una política sobre uno apaga o sube también el otro. La única
// excepción es LINK001, que el parser emite para que el linter lo reciba
// como suyo (findings.SplitParserFindings en las CLIs).
func TestRuleIDs_ParserAndLinterDoNotShareIDs(t *testing.T) {
	parserIDs := ruleIDsIn(t, "../parser", "../internal/elements", "../internal/normalize")
	linterIDs := ruleIDsIn(t, ".")
	shared := map[string]bool{"LINK001": true}
	var clash []string
	for id := range parserIDs {
		if linterIDs[id] && !shared[id] {
			clash = append(clash, id)
		}
	}
	sort.Strings(clash)
	if len(clash) > 0 {
		t.Errorf("rule IDs used by both the parser and the linter: %v", clash)
	}
	for id := range shared {
		if !parserIDs[id] || !linterIDs[id] {
			t.Errorf("%s is documented as shared but is not in both (parser %v, linter %v)", id, parserIDs[id], linterIDs[id])
		}
	}
	if len(parserIDs) < 20 || len(linterIDs) < 20 {
		t.Fatalf("scan found too few IDs (parser %d, linter %d): wrong directories?", len(parserIDs), len(linterIDs))
	}
}
