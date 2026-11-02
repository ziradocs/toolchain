#!/usr/bin/env bash
# Copia LICENSE, NOTICE y los textos de licencia de terceros que viajan
# dentro del binario (core/renderer/chromium/third_party) a
# doclang/internal/licenses/texts y slidelang/internal/licenses/texts, que
# `doclang licenses` y `slidelang licenses` sirven con go:embed. Las copias
# existen porque go:embed no puede leer fuera del módulo; el test de cada
# paquete licenses falla si se desincronizan, y este script las regenera.
set -euo pipefail
root=$(cd "$(dirname "$0")/.." && pwd)
src=$root/core/renderer/chromium/third_party
for module in doclang slidelang; do
  dst=$root/$module/internal/licenses/texts
  rm -rf "$dst"
  mkdir -p "$dst/third_party"
  cp "$root/LICENSE" "$root/NOTICE" "$dst/"
  (cd "$src" && find . -type f \( -name 'LICENSE*' -o -name 'FONT-LICENSE' -o -name 'GUST-FONT-LICENSE.txt' -o -name 'LPPL-1.3c.txt' -o -name 'README' \) | sort) |
    while read -r f; do
      mkdir -p "$dst/third_party/$(dirname "$f")"
      cp "$src/$f" "$dst/third_party/$f"
    done
done
