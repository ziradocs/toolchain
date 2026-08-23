# Motor de temas — plan de arreglos v2

**Fecha:** 23 de agosto de 2026
**Alcance:** `slidelang/internal/generator/css/`, `slidelang/internal/generator/template/assets/js/`, `core/renderer/`
**Estado:** auditado y verificado contra el código; ejecución pendiente

> Este documento cubre **el motor**: la capacidad del formato para tematizar. No contiene ningún
> catálogo de temas — los temas son contenido, viven fuera de este repo y tienen su propia licencia.
> Aquí solo se define qué tiene que saber hacer el intérprete para que **cualquier** tema externo,
> nuestro o de un tercero, funcione.

---

## 1. Por qué esto es del toolchain

Hoy el camino del tema externo está roto, y eso bloquea que los temas puedan vivir fuera del repo.
Un tema distribuido aparte es, por definición, un tema externo: si `ProcessCSSVariables` no lo toca,
su hoja de estilos no tiene efecto. Los cinco arreglos de abajo son la condición para que el sistema
de temas externos —que ya está diseñado (`./themes`, `~/.slidelang/themes`, `SLIDELANG_THEMES_PATH`,
`create_theme.go`, un validador)— sea utilizable de verdad.

Ninguno es una decisión de producto. Son bugs del intérprete: **si el motor no sabe tematizar un
diagrama, ningún tema puede.**

---

## 2. Hallazgos

### 2.1 El CSS de un tema externo no se namespacea — BLOQUEA

`GenerateThemeCSS` normaliza todas las variables a `--slidelang-*` al emitir el `:root`, y el CSS
base pasa por `ProcessCSSVariables`, que reescribe `var(--x)` a `var(--slidelang-x)`.

**El `styles.css` de un tema externo no pasa por ese paso.** `CSSBuilder.Build()` lo escribe crudo
en el bundle (bloque `EXTERNAL THEME CSS`). Efecto medido sobre los temas del repo:

| Tema | `var()` sin prefijo | Efecto |
|---|---|---|
| `modern-blue` | 11 de 11 | Ninguna variable resuelve |
| `startup-tech` | 38 de 38 | Ninguna variable resuelve |
| `startup-tech-solid` | 38 de 38 | Ninguna variable resuelve |
| `aurora-holographic` | 0 de 14 | Correcto |
| `cyberpunk-neon` | 0 de 14 | Correcto |
| `elegant-minimal` | 0 de 17 | Correcto |
| `neomorphism-glass` | 0 de 16 | Correcto |

De aquí sale también el bug de la cita ilegible: `modern-blue` pinta
`blockquote { background: var(--bg-code); color: var(--secondary-color) }` — dos variables muertas.

**Arreglo:** pasar el CSS del tema externo por `ProcessCSSVariables`, y que el validador **rechace**
un `styles.css` con `var()` sin prefijo en vez de aceptarlo en silencio. Los dos, no uno.

### 2.2 El tema no llega a Mermaid, a las gráficas ni a los mapas — BLOQUEA

```js
// template/assets/js/modules/mermaid.js
mermaid.initialize({ theme: 'default', themeVariables: { fontFamily: 'arial' } })

// template/assets/js/modules/charts.js
const defaultColors = ['#3B82F6','#10B981','#F59E0B','#EF4444',
                       '#06B6D4','#8B5CF6','#F97316','#EC4899'];
```

Ninguno consulta el tema. Un diagrama y una gráfica salen idénticos en `cyberpunk-neon` y en
`elegant-minimal`.

Y el CSS que debería sostenerlos casi no existe: `charts.css`, `mermaid.css` y `plantuml.css` usan
**tres variables cada uno**, y las tres son del estado de error (`danger-color`, `bg-danger-light`,
`border-radius`). El diagrama y la gráfica en sí no tienen superficie, rejilla ni etiqueta
tematizadas.

**Arreglo:** resolver el tema antes de emitir el HTML y pasar los tokens ya resueltos a la config JS.
Contrato mínimo que el motor debe exponer, para que un tema externo pueda declararlos:

| Grupo | Tokens que el motor debe leer y propagar |
|---|---|
| Diagrama | `diagram-node-bg`, `diagram-node-fg`, `diagram-node-line`, `diagram-edge`, `diagram-edge-label-bg`, `diagram-cluster-bg`, `diagram-note-bg`, `diagram-accent-bg` |
| Gráfica | `chart-surface`, `chart-grid`, `chart-axis`, `chart-label`, `chart-tooltip-bg`, `chart-cat-1..8`, `chart-seq-1..5` |
| Mapa | `map-surface`, `map-line`, `map-label` |

Mapeo a Mermaid (sustituye el `theme:'default'` fijo):

| Token | `themeVariables` |
|---|---|
| `diagram-node-bg` | `mainBkg`, `primaryColor` |
| `diagram-node-fg` | `primaryTextColor`, `textColor` |
| `diagram-node-line` | `primaryBorderColor`, `nodeBorder` |
| `diagram-edge` | `lineColor` |
| `diagram-edge-label-bg` | `edgeLabelBackground` |
| `diagram-accent-bg` | `secondaryColor` |
| `diagram-cluster-bg` | `clusterBkg`, `clusterBorder` |
| `diagram-note-bg` | `noteBkgColor` |
| `font-main` | `fontFamily` |

**Nota de diseño para quien implemente `chart-cat-*`:** la paleta categórica es la que hace
*identidad de serie*, y su orden fijo de matices es el mecanismo que la mantiene legible bajo
daltonismo. El motor debe tratarla como un set con orden, no como colores intercambiables, y no
debe generar matices nuevos para una serie N+1.

### 2.3 Las fuentes de los temas no se cargan — BLOQUEA

No hay un solo `@font-face` ni enlace a un proveedor de fuentes en todo el toolchain.
`elegant-minimal` declara *Playfair Display*, *Crimson Text* y *Berkeley Mono*; en cualquier máquina
que no las tenga instaladas cae a Georgia y Monaco **en silencio**.

Un sistema de temas cuyo segundo eje es tipografía no puede depender de lo que el espectador tenga
instalado.

**Arreglo:** que un tema pueda declarar sus fuentes y que el motor las resuelva. Decisión abierta:
auto-hospedarlas en el bundle (pesa, funciona sin red, es lo correcto para `--embed-assets`) o
enlazarlas. Para `--embed-assets` la primera es la única opción coherente.

### 2.4 PlantUML no se tematiza con CSS — DECIDIR

`core/renderer/plantuml_encoder.go` comprime el fuente y arma una URL contra
`https://www.plantuml.com/plantuml`; hay también un `kroki_fetcher` con servidor configurable. En
los dos casos vuelve **una imagen ya pintada**.

- El tema tiene que entrar como `skinparam` en el fuente, **antes** de codificar:
  `backgroundColor`, `defaultFontName`, `ArrowColor`, `BorderColor`, `BackgroundColor`, `FontColor`.
- Cambiar de modo invalida la imagen cacheada: hay que versionar la URL por `(tema, modo)`.
- Sin un Kroki propio no hay modo offline real.
- Para un producto que vende auditoría y retención, que el diagrama de arquitectura de un cliente
  viaje a un servidor público merece una decisión explícita, no un default.

Es el arreglo más caro de los tres mecanismos y el que más se subestima al planear.

### 2.5 El modo es un tema aparte, no una variante — DECIDIR

`ResolveTheme` trabaja con un nombre plano y `dark` es una entrada más del catálogo. Con un catálogo
de N temas eso da 2N nombres en el selector y el autor elige mal.

**Propuesta:** `theme: <nombre>` + `mode: light|dark` en el frontmatter, con la variante clara como
respaldo automático en `@media print` — un tema oscuro exportado a PDF hoy imprime en negro sólido.

### 2.6 Dos sistemas de variables conviviendo — DECIDIR

Los temas de `slidelang/themes/` declaran `--slidelang-*`; los embebidos en Go declaran `--*` y se
normalizan al vuelo. Funciona, pero es la razón por la que un autor de temas escribe el prefijo mal.

---

## 3. Los tres mecanismos de tematización

Presupuestar los tres por separado. Meterlos en un solo issue "tematizar los bloques" garantiza que
el tercero se quede fuera.

| Clase | Bloques | Cómo se tematiza | Estado |
|---|---|---|---|
| **A · CSS** | código y syntax, tablas, citas, callouts, checklists, math (MathJax emite SVG y hereda `currentColor`) | Tokens CSS en el `:root` | Funciona, salvo §2.1 |
| **B · JS en el cliente** | Mermaid, Chart.js, mapas | Inyectar los tokens resueltos en la config al iniciar | No existe (§2.2) |
| **C · Imagen remota** | PlantUML | `skinparam` en el fuente antes de codificar | No existe (§2.4) |

---

## 4. Contrato del tema

Lo que el validador debe exigir a cualquier tema, propio o de terceros:

1. **Cero hex en `styles.css`.** Todo resuelve a token con prefijo `--slidelang-`. Un `var()` sin
   prefijo es un error de validación, no una advertencia.
2. **Los seis ejes declarados** — paleta semántica, par tipográfico, retícula, layouts, bloques
   especiales (incluidos los grupos de §2.2) y cromo.
3. **Las dos variantes o una declaración explícita** de que el tema es de un solo modo.
4. **Respaldo de impresión** en todo tema oscuro.
5. **Pisos de contraste** — 4.5 para texto, 3.0 para acento sobre fondo y comentarios de código. El
   linter ya tiene el seam (`theme-color`, issue #30); falta cablearlo a CI.

---

## 5. Los siete temas del repo

Hoy `slidelang/themes/embed.go` los embebe **solo para el build WASM**. El CLI instalado con
`go install` no los trae: los busca en disco en `./themes` y `~/.slidelang/themes`. Es decir, ya son
assets externos de facto — nadie que instale el CLI tiene `modern-blue`.

**Decisión:** el repo conserva un único tema de referencia neutro y sin marca (`default`), para que
`slidelang build` funcione solo, sin nada instalado, y para que sea el fallback cuando un `.slidelang`
declara un tema que no está presente — comportamiento que `ResolveTheme` ya implementa: cae a
`default` con un aviso y nunca devuelve nil.

Los siete actuales se retiran. Un deck con `theme: modern-blue` debe seguir compilando: fallback a
`default` con aviso del linter, que es lo que ya pasa.

---

## 6. Orden de ejecución

```
1 Namespacing del CSS externo            §2.1   BLOQUEA — habilita todo lo demás
2 Carga de fuentes                       §2.3   BLOQUEA
3 Clase B: tokens -> Mermaid / charts    §2.2   BLOQUEA
4 Clase C: skinparam en PlantUML         §2.4   caro, decisión de infra pendiente
5 theme + mode, respaldo @media print    §2.5
6 Unificación del prefijo                §2.6
7 Validador estricto                     §4     depende de 1..6
8 Retiro de los siete temas              §5     depende de 7
9 Contraste en CI                        §4.5
```

El 1 es la puerta: mientras no esté, un tema externo no puede funcionar y no tiene sentido escribir
ninguno.

### Criterio de terminado

Un tema colocado en `./themes/<nombre>/` con `theme.json` + `styles.css`, sin un solo hex escrito a
mano, debe producir un deck donde el bloque de código, la tabla, la cita, el diagrama Mermaid, la
gráfica y el diagrama PlantUML respeten sus tokens, en los dos modos, con las fuentes declaradas
cargando de verdad, y con el contraste verificado en CI.

---

## 7. Lo que no entra

- Ningún catálogo de temas. Son contenido, viven fuera de este repo y bajo su propia licencia.
- Los temas de `doclang` (`professional`, `academic`, `technical`, `page-view`). `doclang` tiene su
  propio generador y no comparte `internal/generator/css/`, así que estos arreglos **no** lo
  benefician automáticamente. Si se quiere el mismo contrato allá, es otro trabajo.
