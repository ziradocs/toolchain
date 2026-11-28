// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package anchors

import (
	"reflect"
	"testing"
)

// La regla de colisión: el primero conserva el id base, los siguientes toman
// el primer sufijo -N (N >= 2) libre, probado contra todo lo usado, incluidos
// los ids base reales y los reservados.
func TestSetUnique(t *testing.T) {
	for _, tc := range []struct {
		name     string
		reserved []string
		bases    []string
		want     []string
	}{
		{"distintos", nil, []string{"a", "b"}, []string{"a", "b"}},
		{"repetidos", nil, []string{"intro", "intro", "intro"}, []string{"intro", "intro-2", "intro-3"}},
		// El tercero tiene id base notas-2, que ya tomó el segundo.
		{"sufijo contra id base real", nil, []string{"notas", "notas", "notas-2"}, []string{"notas", "notas-2", "notas-2-2"}},
		// Al revés: el id base real llega primero y el repetido lo salta.
		{"id base real primero", nil, []string{"notas-2", "notas", "notas"}, []string{"notas-2", "notas", "notas-3"}},
		{"reservado", []string{"notas"}, []string{"notas", "notas"}, []string{"notas-2", "notas-3"}},
		{"reservado con sufijo", []string{"notas-2"}, []string{"notas", "notas"}, []string{"notas", "notas-3"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var s Set
			for _, r := range tc.reserved {
				s.Reserve(r)
			}
			var got []string
			for _, b := range tc.bases {
				if d := s.Derive(b); d != s.Derive(b) {
					t.Fatalf("Derive no es estable: %q", d)
				}
				want := s.Derive(b)
				a := s.Unique(b)
				if a != want {
					t.Errorf("Unique(%q) = %q, Derive dijo %q", b, a, want)
				}
				got = append(got, a)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// next es solo un atajo: con muchos repetidos el resultado es el mismo que
// probar desde -2 cada vez.
func TestSetUniqueManyRepeats(t *testing.T) {
	var s Set
	s.Reserve("x-5")
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		a := s.Unique("x")
		if seen[a] || a == "x-5" {
			t.Fatalf("anchor repetido %q en la vuelta %d", a, i)
		}
		seen[a] = true
	}
}

func TestDocumentBase(t *testing.T) {
	if got := DocumentBase(""); got != DocumentFallback {
		t.Errorf("DocumentBase(\"\") = %q, want %q", got, DocumentFallback)
	}
	if got := DocumentBase("intro"); got != "intro" {
		t.Errorf("DocumentBase(\"intro\") = %q", got)
	}
}
