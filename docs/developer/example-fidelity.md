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
baseline con sus conteos exactos (archivo, chequeo, `{source, html}`). La
comparación real es sobre el **gap** (`abs(source - html)`), no sobre el par
exacto: un mismatch falla el chequeo solo si su gap es nuevo (no estaba en el
baseline) o si empeoró respecto al gap baselineado. Un PR que reduce el gap
sin cerrarlo del todo (p.ej. arregla la mitad de las imágenes que faltaban)
no falla — el script lo marca como "mejorado, se puede ajustar el baseline"
en vez de como fallo. Arreglar el bug de fondo del todo (gap llega a 0) deja
esa entrada "stale" — el script lo avisa en ambos casos; hay que borrarla (o
ajustarla) a mano del JSON, nunca declararla exenta para siempre.

Nota para PRs que editan la fixture misma (no solo el renderer): si un PR
cambia el `.slidelang`/`.doclang` de forma que el conteo del **fuente**
cambia (p.ej. reescribir `POINTS` como `CHECKLIST`), la entrada baselineada
para ese archivo queda sin sentido — hay que borrarla explícitamente, no
solo dejar que el script la relaje.

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
