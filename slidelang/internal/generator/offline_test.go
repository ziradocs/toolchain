// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import "testing"

func excludeListHas(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// TestBuildExcludeList_OfflineExcludesInteractiveModules: en modos offline se
// excluyen los módulos JS client-side de mermaid/charts/maps (issue #92).
func TestBuildExcludeList_OfflineExcludesInteractiveModules(t *testing.T) {
	g := &Generator{}
	for _, mode := range []string{"offline-assets", "offline-inline"} {
		got := g.buildExcludeList(GeneratorOptions{RenderMode: mode})
		for _, want := range []string{"mermaid", "charts", "maps"} {
			if !excludeListHas(got, want) {
				t.Errorf("mode %q: buildExcludeList should exclude %q, got %v", mode, want, got)
			}
		}
	}
}

// TestBuildExcludeList_BrowserKeepsInteractiveModules: en modo browser NO se
// excluyen (se renderizan client-side contra CDNs).
func TestBuildExcludeList_BrowserKeepsInteractiveModules(t *testing.T) {
	g := &Generator{}
	for _, mode := range []string{"browser", ""} {
		got := g.buildExcludeList(GeneratorOptions{RenderMode: mode})
		for _, notWant := range []string{"mermaid", "charts", "maps"} {
			if excludeListHas(got, notWant) {
				t.Errorf("mode %q should not exclude %q, got %v", mode, notWant, got)
			}
		}
	}
}
