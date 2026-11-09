// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"net/url"
	"os"
	"strings"

	"go.ziradocs.com/core/v2/ast"
	"go.ziradocs.com/core/v2/renderer"
	"go.ziradocs.com/core/v2/util"
)

// headerLogos son los logos de encabezado declarados en header: y en
// layout_defaults:.
func headerLogos(hf *ast.HeaderFooterConfig) []*ast.LogoConfig {
	if hf == nil {
		return nil
	}
	var logos []*ast.LogoConfig
	if hf.Header != nil && hf.Header.Logo != nil {
		logos = append(logos, hf.Header.Logo)
	}
	for _, l := range hf.LayoutDefaults {
		if l != nil && l.Header != nil && l.Header.Logo != nil {
			logos = append(logos, l.Header.Logo)
		}
	}
	return logos
}

// checkHeaderLogos reporta, para FailOnUnavailable, cada logo de
// encabezado que no se puede mostrar, con la misma regla en todos los
// formatos (antes solo lo veía el HTML page-view, el único que dibuja el
// encabezado en el cuerpo): un logo local que no existe o queda fuera de
// AssetRoot, y uno remoto donde la salida no lo pediría (siempre en DOCX, que
// no descarga imágenes; en PDF y HTML offline sin AllowRemoteImages). Se
// reporta con línea 0 y la fuente tal como se escribió, igual que core, así
// que el collector no lo repite si core también lo vio.
func checkHeaderLogos(hf *ast.HeaderFooterConfig, variables map[string]interface{}, opts GeneratorOptions, add func(kind string, line int, detail string)) {
	for _, logo := range headerLogos(hf) {
		raw := strings.TrimSpace(renderer.ProcessVariables(logo.Source, variables))
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil {
			add("image", 0, logo.Source)
			continue
		}
		if u.Scheme != "" || u.Host != "" {
			scheme := strings.ToLower(u.Scheme)
			remote := scheme == "http" || scheme == "https" || (scheme == "" && u.Host != "")
			if !remote {
				continue // data: u otro esquema que no se descarga
			}
			buildTime := opts.Format == "docx" || opts.Format == "pdf" || opts.ImageMode == "offline-inline" || opts.ImageMode == "offline-assets"
			if opts.Format == "docx" || (buildTime && !opts.AllowRemoteImages) {
				add("image", 0, logo.Source)
			}
			continue
		}
		root := opts.AssetRoot
		if root == "" {
			if cwd, err := os.Getwd(); err == nil {
				root = cwd
			}
		}
		resolved, err := util.ResolveConfinedPathFrom(root, opts.SourceDir, renderer.LocalPathFromURL(raw))
		if err != nil {
			add("image", 0, logo.Source)
			continue
		}
		if info, err := os.Stat(resolved); err != nil || info.IsDir() {
			add("image", 0, logo.Source)
		}
	}
}
