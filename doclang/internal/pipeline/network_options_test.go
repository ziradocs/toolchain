// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package pipeline

import (
	"strings"
	"testing"

	"go.ziradocs.com/core/v2/renderer"
)

// TestValidateNetworkOptions: --no-network no se combina con lo que sale a
// la red, y --tile-server tiene que ser un template válido que este build
// sepa usar.
func TestValidateNetworkOptions(t *testing.T) {
	if err := ValidateNetworkOptions(false, false, false, "", ""); err != nil {
		t.Fatalf("defaults rejected: %v", err)
	}
	if err := ValidateNetworkOptions(true, false, false, "", ""); err != nil {
		t.Fatalf("--no-network alone rejected: %v", err)
	}
	for _, c := range []struct {
		name                   string
		noNet, remote, install bool
		want                   string
	}{
		{"remote images", true, true, false, "--allow-remote-images"},
		{"install chromium", true, false, true, "--install-chromium"},
	} {
		err := ValidateNetworkOptions(c.noNet, c.remote, c.install, "", "")
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: err = %v, want mention of %s", c.name, err, c.want)
		}
	}

	_, supported := any(renderer.MapConfig{}).(interface {
		WithTileServer(string, string) (renderer.MapConfig, error)
	})
	err := ValidateNetworkOptions(false, false, false, "https://tiles.example.com/{z}/{x}/{y}.png", "ua")
	if supported && err != nil {
		t.Errorf("valid tile server rejected: %v", err)
	}
	if !supported && (err == nil || !strings.Contains(err.Error(), "--tile-server needs a core")) {
		t.Errorf("old core: err = %v, want the needs-core error", err)
	}
	if err := ValidateNetworkOptions(false, false, false, "ftp://tiles.example.com/{z}/{x}/{y}", ""); err == nil {
		t.Error("ftp tile server accepted")
	}
}
