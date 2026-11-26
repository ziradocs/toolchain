// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

package generator

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	docx "github.com/mmonterroca/docxgo/v2"
	"github.com/mmonterroca/docxgo/v2/domain"
	"go.ziradocs.com/core/v2/renderer"
)

// Los links del .docx son hipervínculos de Word: un destino absoluto
// (https:, mailto:…) es un w:hyperlink externo, con su relación en el .rels,
// y uno interno (#ancla) es un w:hyperlink con w:anchor hacia el marcador del
// encabezado cuyo id en el HTML es esa ancla. Antes solo se pintaban con el
// color y el subrayado de un link, sin destino: un comentario decía que
// docxgo no lo soportaba, pero desde v2.x NewHyperlinkField + Run.AddField
// escriben los dos.
//
// Lo que docxgo no expone es nombrar un marcador. Al escribir el documento
// (generateHeadingBookmarks) le pone a cada párrafo con estilo Heading* del
// cuerpo, en orden, un marcador _Toc<n> desde 0 (en un documento nuevo), y
// pisa cualquier otro. Así que un link interno no puede apuntar a un
// marcador propio: apunta al _Toc<n> que le va a tocar a su encabezado, que
// resolveAnchorLinks calcula con la misma regla justo antes de guardar.
// TestDOCXInternalLinksTargetTheirHeading abre el .docx y comprueba que cada
// w:anchor sea el w:bookmarkStart del encabezado correcto: si una versión de
// docxgo cambia la regla, esa prueba falla en vez de dejar links rotos.

// docxAnchorLink es un run de la etiqueta de un link interno que espera su
// marcador.
type docxAnchorLink struct {
	run    domain.Run
	anchor string
}

// docxHeadingAnchor es el ancla (el id del encabezado en el HTML) de un
// párrafo de encabezado del cuerpo.
type docxHeadingAnchor struct {
	para   domain.Paragraph
	anchor string
}

// docxURLScheme reconoce un destino con esquema (RFC 3986). Un destino
// relativo (docs/x.md) no sale como hipervínculo: en Word se resolvería
// contra la carpeta donde se abra el .docx, no contra la del documento.
var docxURLScheme = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)

// linkRun convierte r, un run de la etiqueta de un link a dest (ya validado
// por docxApplyLink), en hipervínculo. Un link interno queda pendiente hasta
// resolveAnchorLinks. Si docxgo rechaza el campo (un destino con comillas
// dobles, un run sin relaciones), el run se queda como texto con estilo de
// link, como antes.
func (g *DOCXGenerator) linkRun(r domain.Run, dest string) {
	d := strings.TrimSpace(renderer.DecodeLinkDestination(dest))
	if anchor, ok := strings.CutPrefix(d, "#"); ok {
		if anchor != "" {
			g.anchorLinks = append(g.anchorLinks, docxAnchorLink{run: r, anchor: anchor})
		}
		return
	}
	if !docxURLScheme.MatchString(d) || r.Text() == "" {
		return
	}
	if err := r.AddField(docx.NewHyperlinkField(d, r.Text())); err != nil {
		g.logger.Debug("DOCX", "link to %q left as styled text: %v", d, err)
	}
}

// recordHeadingAnchor anota el ancla del encabezado que acaba de escribirse
// en p.
func (g *DOCXGenerator) recordHeadingAnchor(p domain.Paragraph, anchor string) {
	if anchor != "" {
		g.headingAnchors = append(g.headingAnchors, docxHeadingAnchor{para: p, anchor: anchor})
	}
}

// resolveAnchorLinks le da a cada link interno pendiente el marcador del
// primer encabezado con esa ancla (como un navegador con dos id iguales).
// Primero se busca el ancla tal cual; si no hay, la que resulta de derivarla
// como el id de un encabezado (renderer.DeriveAnchor, sobre el ancla ya
// decodificada): así [x](#sección-dos), [x](#Seccion-Dos) o
// [x](#secci%C3%B3n-dos), como los escribe un Markdown pensado para GitHub,
// llegan al encabezado "Sección dos" (id seccion-dos). Un ancla sin
// encabezado (una figura, una tabla, un id que no existe) se queda como
// texto con estilo de link, con un aviso: un w:anchor sin marcador no lleva a
// ningún lado.
func (g *DOCXGenerator) resolveAnchorLinks(doc domain.Document) {
	if len(g.anchorLinks) == 0 {
		return
	}
	anchorOf := make(map[domain.Paragraph]string, len(g.headingAnchors))
	for _, h := range g.headingAnchors {
		anchorOf[h.para] = h.anchor
	}
	bookmarks := map[string]string{}
	n := 0
	for _, p := range doc.Paragraphs() {
		styled, ok := p.(interface{ StyleName() string })
		if !ok || !strings.HasPrefix(styled.StyleName(), "Heading") {
			continue
		}
		name := fmt.Sprintf("_Toc%d", n)
		n++
		if a, ok := anchorOf[p]; ok {
			if _, taken := bookmarks[a]; !taken {
				bookmarks[a] = name
			}
		}
	}
	unresolved := map[string]bool{}
	for _, l := range g.anchorLinks {
		name, ok := bookmarks[l.anchor]
		if !ok {
			decoded := l.anchor
			if u, err := url.PathUnescape(decoded); err == nil {
				decoded = u
			}
			name, ok = bookmarks[renderer.DeriveAnchor(decoded)]
		}
		if !ok {
			if !unresolved[l.anchor] {
				unresolved[l.anchor] = true
				g.logger.Warn("DOCX: link to #%s left as text: no heading has that anchor", l.anchor)
			}
			continue
		}
		if l.run.Text() == "" {
			continue
		}
		if err := l.run.AddField(docx.NewHyperlinkField("#"+name, l.run.Text())); err != nil {
			g.logger.Debug("DOCX", "link to #%s left as styled text: %v", l.anchor, err)
		}
	}
}

// docxHeadingTags quita las etiquetas del HTML de un encabezado.
var docxHeadingTags = regexp.MustCompile(`<[^>]+>`)

// headingAnchorFor es el id que el HTML le da al encabezado de content (un
// <hN id="…"> ya renderizado), o el que deriva de su texto si no trae id,
// como addIDsToHeaders en core.
func headingAnchorFor(content, text string) string {
	if m := headingIDPattern.FindStringSubmatch(content); m != nil {
		return m[1]
	}
	return renderer.DeriveAnchor(docxHeadingTags.ReplaceAllString(text, ""))
}
