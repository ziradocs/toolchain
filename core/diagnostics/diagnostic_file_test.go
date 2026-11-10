// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package diagnostics

import (
	"encoding/json"
	"strings"
	"testing"
)

// File es opcional: sin él el JSON no cambia; con él aparece como "file" y en
// String() delante de la posición.
func TestDiagnostic_FileIsOptional(t *testing.T) {
	d := NewError("m", NewPosition(3, 1), "parser")
	b, _ := json.Marshal(d)
	if strings.Contains(string(b), `"file"`) {
		t.Errorf("no file field expected: %s", b)
	}
	if d.String() != "error: m (3:1)" {
		t.Errorf("String() %q", d.String())
	}
	d.File = "/x/part.doclang"
	b, _ = json.Marshal(d)
	if !strings.Contains(string(b), `"file":"/x/part.doclang"`) {
		t.Errorf("file field expected: %s", b)
	}
	if d.String() != "error: m (/x/part.doclang:3:1)" {
		t.Errorf("String() %q", d.String())
	}
}
