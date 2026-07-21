// Copyright 2026 Misael Monterroca
// SPDX-License-Identifier: Apache-2.0

// Package slidelangcore es el motor compartido de parsing, AST y
// renderizado que usan los CLIs slidelang y doclang. No está pensado para
// ser importado directamente por terceros — ver la política de
// estabilidad abajo.
//
// # Modelo de consumo
//
// SlideLang/DocLang se diseñaron para invocarse como ejecutables, no como
// librería embebida. Un archivo .slidelang o .doclang se procesa con:
//
//	slidelang build presentacion.slidelang --format html
//	doclang build documento.doclang --format json
//
// # Contratos públicos estables
//
// Lo que este proyecto promete mantener y versionar:
//
//  1. La interfaz de línea de comandos de slidelang/doclang: subcomandos,
//     flags, formatos de entrada (.slidelang, .doclang) y formatos de
//     salida (html, json, pdf, docx, markdown).
//
//  2. El AST serializado vía --format json, versionado semver por
//     ast.SchemaVersion (ver schema/ast.schema.json en la raíz del
//     monorepo, fuera de este módulo, y el paquete npm
//     @ziradocs/ast-types). Este es el contrato recomendado para
//     integraciones de terceros: agentes que generan SlideLang, el
//     visor web, o cualquier consumidor externo del árbol de contenido.
//     Ver docs/architecture/json-ast-contract.md para el detalle campo
//     por campo.
//
//  3. Un futuro entrypoint WASM (issue #134) para ejecutar el parser y
//     el renderer directamente en el navegador, como wrapper sobre este
//     mismo módulo.
//
// La estructura HTML generada y sus clases CSS NO son parte de este
// contrato y pueden cambiar entre releases sin aviso — ver
// docs/architecture/json-ast-contract.md.
//
// # La API de Go es un detalle de implementación interno
//
// Los paquetes de este módulo (ast, parser, renderer, elements internos,
// config, util, etc.) están diseñados para ser consumidos por
// slidelang y doclang — los dos únicos importadores previstos —
// no por terceros. No hay compromiso de estabilidad semver sobre
// ninguna firma, tipo o función exportada de Go: pueden renombrarse,
// removerse o cambiar de comportamiento en cualquier versión menor.
//
// El módulo se versiona v0.x deliberadamente (convención de Go para "sin
// promesa de compatibilidad de API"). Si en el futuro surge una
// necesidad real de embeber este motor directamente desde otro programa
// Go, se puede curar y versionar un subconjunto estable en ese
// momento — promover un símbolo de inestable a estable no rompe a nadie;
// lo inverso sí.
//
// Como parte de esta política, los paquetes ai/ y elements/ viven bajo
// internal/ precisamente porque ninguno de los dos CLIs los importa
// directamente (solo parser los usa internamente) — el compilador de Go
// impide que un módulo externo los importe.
package slidelangcore
