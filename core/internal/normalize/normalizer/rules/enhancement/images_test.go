// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package enhancement

import "testing"

// Una URL, una ruta relativa al protocolo o una ruta desde la raíz no es una
// ruta placeholder: ImagesRule la deja como está. Solo una ruta relativa va a
// assets/images/<nombre>.
func TestImagesRule_KeepsExternalAndAbsolutePaths(t *testing.T) {
	r := NewImagesRule()
	cases := []struct {
		name, path, want string
	}{
		{"https", "https://upload.wikimedia.org/wikipedia/commons/4/47/a.png", "https://upload.wikimedia.org/wikipedia/commons/4/47/a.png"},
		{"images en el host", "https://images.unsplash.com/photo-1.jpg", "https://images.unsplash.com/photo-1.jpg"},
		{"esquema en mayúsculas", "HTTP://example.com/sample.gif", "HTTP://example.com/sample.gif"},
		{"data URI", "data:image/svg+xml;utf8,x.svg", "data:image/svg+xml;utf8,x.svg"},
		{"relativa al protocolo", "//cdn.example.com/x.png", "//cdn.example.com/x.png"},
		{"desde la raíz", "/api/assets/a/b.png", "/api/assets/a/b.png"},
		{"relativa", "img/x.png", "assets/images/x.png"},
		{"relativa placeholder", "ruta/imagen.png", "assets/images/imagen.png"},
		{"ya en assets", "assets/logo.png", "assets/logo.png"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.Apply("![a](" + tc.path + ")")
			if err != nil {
				t.Fatal(err)
			}
			if want := "![a](" + tc.want + ")"; got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// En una misma línea, la URL queda y la ruta relativa de al lado se reescribe.
func TestImagesRule_MixedLine(t *testing.T) {
	got, _ := NewImagesRule().Apply("![u](https://images.unsplash.com/p.jpg) ![r](img/x.png) ![s](/api/assets/a/b.png)")
	want := "![u](https://images.unsplash.com/p.jpg) ![r](assets/images/x.png) ![s](/api/assets/a/b.png)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
