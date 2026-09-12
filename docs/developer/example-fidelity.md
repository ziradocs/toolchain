# Chequeo de fidelidad de `examples/`

`scripts/check-example-fidelity.py` compila cada `.slidelang`/`.doclang` bajo
`examples/` y compara conteos estructurales del **fuente** (charts, mapas,
mermaid, plantuml, tablas y sus filas, imágenes, checklist items, code
fences, special blocks por tipo, grids, code-groups) contra los mismos
conteos en el **HTML generado**. `html-validate.yml`/`ci.yml` verifican que
el corpus compile y que el HTML sea válido, pero ninguno de los dos atrapa
un ejemplo que compila y produce HTML válido mientras pierde o garabatea
contenido en silencio — un chart con el eje colapsado, una imagen que
desaparece de una celda, un `:::code-group` cuyos paneles quedan apilados
sin CSS. Es este chequeo el que corre en el job `example-fidelity` de
`html-validate.yml`.

## Correrlo localmente

Desde la raíz del repo, con un `go.work` local (`use (./core ./doclang
./slidelang)`) o publicado:

```bash
python3 scripts/check-example-fidelity.py
```

Compila `slidelang`/`doclang` a un directorio temporal, genera el HTML de
los 105 ejemplos y compara. Sale con `0` si no hay mismatches nuevos, `1` si
los hay.

## El baseline

`scripts/check-example-fidelity-baseline.json` sigue el mismo patrón que
`scripts/check-example-assets-baseline.txt`: un mismatch que ya existía
cuando se escribió el chequeo no bloquea CI por sí solo — se registra en el
baseline con sus conteos exactos (archivo, chequeo, `{source, html}`). Un
mismatch **falla** el chequeo solo si es nuevo o si sus conteos empeoraron
respecto al baseline. Arreglar el bug de fondo dejará esa entrada "stale" (el
script lo avisa) — hay que borrarla a mano del JSON, nunca declararla exenta
para siempre.

Para regenerar el baseline completo (por ejemplo, tras arreglar varios de los
bugs que encontró) hazlo por partes: arregla el bug, corre el script, y
confirma que la entrada correspondiente desapareció como "stale" antes de
borrarla — no lo regeneres a ciegas con `--write-baseline`, porque eso
también capturaría cualquier regresión nueva como si fuera baseline legítimo.

```bash
python3 scripts/check-example-fidelity.py --write-baseline
```

## Qué NO valida

Es un chequeo **estructural** (conteos), no semántico ni visual: dos `<tr>`
con el contenido de las celdas cambiado de orden siguen contando igual. No
reemplaza una revisión visual real en navegador para un ejemplo canónico
nuevo o editado a fondo.
