// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package report

import (
	"encoding/json"
	"testing"

	"go.ziradocs.com/core/v2/diagnostics"
)

// Un diagnóstico con File (un archivo traído con @include) va en SARIF con ese
// archivo en artifactLocation; el resto, con el documento.
func TestSARIF_ArtifactLocationUsesDiagnosticFile(t *testing.T) {
	inDoc := diagnostics.NewError("en el documento", diagnostics.NewPosition(3, 1), "parser")
	inPart := diagnostics.NewError("en el incluido", diagnostics.NewPosition(2, 1), "parser")
	inPart.File = "/docs/part.doclang"
	data, err := generateSARIF([]diagnostics.Diagnostic{inDoc, inPart}, nil, "main.doclang", nil)
	if err != nil {
		t.Fatal(err)
	}
	var out SARIF
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, r := range out.Runs[0].Results {
		got = append(got, r.Locations[0].PhysicalLocation.ArtifactLocation.Uri)
	}
	if len(got) != 2 || got[0] != "main.doclang" || got[1] != "/docs/part.doclang" {
		t.Errorf("artifactLocation uris %v", got)
	}
}
