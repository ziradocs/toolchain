// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package elements

import (
	"reflect"
	"testing"
)

func TestTableParser_MarkdownColumnAlign(t *testing.T) {
	tests := []struct {
		name  string
		delim string
		want  []string
	}{
		{"declared", "|:---|:---:|---:|---|", []string{"left", "center", "right", ""}},
		{"none declared stays nil", "|---|---|---|---|", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			table, _ := parseTableBlockAt(t, "flex",
				"| A | B | C | D |",
				tt.delim,
				"| 1 | 2 | 3 | 4 |",
			)
			if !reflect.DeepEqual(table.ColumnAlign, tt.want) {
				t.Errorf("ColumnAlign = %#v, want %#v", table.ColumnAlign, tt.want)
			}
			if len(table.Rows) != 1 {
				t.Errorf("delimiter row leaked into data: %v", table.Rows)
			}
		})
	}
}
