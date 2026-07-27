#!/usr/bin/env bash
# Automatiza el "dance" de release core→CLIs (issue #25): cortar un tag
# core/vX.Y.Z, esperar a que el proxy de Go + sum.golang.org lo indexen, y
# bumpear el require de slidelang/go.mod + doclang/go.mod a esa versión,
# dejando todo listo para abrir la PR de los CLIs.
#
# A diferencia de scripts/release.sh (que usa `set -e` pelado), este script
# usa `set -euo pipefail` — la convención mayoritaria del repo
# (playground/build.sh, ci.yml, chromium-pin-check.yml) y la más segura para
# un script que corre `go get`/`go mod tidy` en loop.
#
# Uso:
#   scripts/bump-core.sh vX.Y.Z [ref]
#
#   vX.Y.Z  — versión a taggear como core/vX.Y.Z (sin el prefijo "core/").
#   ref     — commit/rama a taggear (default: origin/main). El commit debe
#             tener core/ ya mergeado con los cambios que se quieren
#             publicar; este script NO mergea nada.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

VERSION=${1:-}
REF=${2:-origin/main}

if [[ -z "$VERSION" ]]; then
  echo "Error: Debes proveer una versión (ej. v2.2.1)"
  echo "Uso: scripts/bump-core.sh vX.Y.Z [ref]"
  exit 1
fi

# 1. Validar SemVer — más estricto que release.sh: ancla ambos extremos, sin
#    sufijo -prerelease/+build ni basura al final (release.sh's regex deja
#    pasar "v1.2.3-cualquier-cosa!!"; un tag de módulo Go puro no lo necesita).
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Error: La versión debe seguir SemVer exacto y empezar con 'v' (ej. v2.2.1)"
  exit 1
fi

cd "$REPO_ROOT"

# 2. Validar el sufijo /v2 (o el que corresponda) contra core/go.mod — mismo
#    check que release.sh:22-32, reusado verbatim: un tag mayor sin el
#    sufijo correspondiente en el module path de Go rompe la resolución de
#    versión para siempre (ver CLAUDE.md sobre core/slidelang/doclang v2.1.0).
MAJOR_VERSION=$(echo "$VERSION" | grep -o '^v[0-9]*')
if [[ "$MAJOR_VERSION" != "v1" && "$MAJOR_VERSION" != "v0" ]]; then
  MODULE_VER=$(grep "^module" core/go.mod | awk -F'/' '{print $NF}')
  if [[ "$MODULE_VER" != "$MAJOR_VERSION" ]]; then
    echo "🔥 Error Crítico de Go: Estás intentando lanzar $VERSION, pero core/go.mod no tiene el sufijo /$MAJOR_VERSION."
    echo "Si vas a sacar una versión mayor nueva, debes actualizar los go.mod y todos los imports primero."
    exit 1
  fi
fi

# Árbol limpio — mismo check que release.sh:16-20. Este script va a bumpear
# go.mod/go.sum de slidelang y doclang más abajo; empezar con cambios sin
# commitear haría imposible distinguir "esto lo hizo el script" de "esto ya
# estaba" al revisar el diff final.
if [[ -n $(git status -s) ]]; then
  echo "Error: Tienes cambios sin commitear. Haz commit o stash primero."
  exit 1
fi

git fetch origin --tags -q

# 3. Rechazar si el tag ya existe — el gap que release.sh no cubre y que
#    forzó core/v2.1.3 (colisión con un tag intermedio ya cortado). Se
#    chequea el REMOTO, no solo local: `git tag -l` no ve un tag que existe
#    en origin pero no se fetcheó todavía a este checkout — exactamente el
#    estado que produjo esa colisión la primera vez.
if git ls-remote --tags origin "refs/tags/core/$VERSION" | grep -q .; then
  echo "Error: el tag core/$VERSION ya existe en origin. Elegí otra versión."
  exit 1
fi

TAG_COMMIT=$(git rev-parse "$REF")
echo "🚀 Taggeando core/$VERSION en $REF ($TAG_COMMIT)..."

# 4. Crear y pushear SOLO core/vX.Y.Z — nunca un tag pelado vX.Y.Z.
#    .github/workflows/release.yml dispara con "v[0-9]*.[0-9]*.[0-9]*" Y con
#    un "v*" suelto (ver el comentario en ese archivo sobre por qué el "v*"
#    ahí es un riesgo real, no solo teórico); "core/vX.Y.Z" no empieza con
#    "v" así que ninguno de los dos patrones matchea — confirmado, esto NO
#    dispara goreleaser.
git tag -a "core/$VERSION" "$TAG_COMMIT" -m "core/$VERSION"
git push origin "refs/tags/core/$VERSION"

# 5. Calentar proxy.golang.org + sum.golang.org antes de intentar `go get`.
#    Existe por evidencia, no por precaución teórica: un fetch inmediato
#    después de pushear un tag nuevo puede pegarle a sum.golang.org con un
#    500 transitorio hasta que el proxy lo indexa.
MODULE_PATH="go.ziradocs.com/core/v2"
echo "☁️ Calentando proxy.golang.org + sum.golang.org para $MODULE_PATH@$VERSION..."
warmed=false
for attempt in $(seq 1 12); do
  info_code=$(curl -s -o /dev/null -w "%{http_code}" "https://proxy.golang.org/${MODULE_PATH}/@v/${VERSION}.info" || echo "000")
  sum_code=$(curl -s -o /dev/null -w "%{http_code}" "https://sum.golang.org/lookup/${MODULE_PATH}@${VERSION}" || echo "000")
  if [[ "$info_code" == "200" && "$sum_code" == "200" ]]; then
    warmed=true
    break
  fi
  echo "  intento $attempt/12: proxy=$info_code sumdb=$sum_code, reintentando en 5s..."
  sleep 5
done
if [[ "$warmed" == true ]]; then
  echo "✅ proxy.golang.org y sum.golang.org ya sirven $VERSION."
else
  echo "⚠️ El proxy/sumdb no confirmó $VERSION tras 12 intentos (60s) — se sigue igual; el paso 6 tiene su propio fallback."
fi

# 6. Bumpear el require en slidelang/go.mod y doclang/go.mod, vía GOWORK=off
#    (la resolución real de CI/go install — go.work local no debe enmascarar
#    esto). Primero el camino normal; si el sumdb sigue teniendo lag pese al
#    warming de arriba, GOPROXY=direct GOSUMDB=off como fallback (esto pasó
#    de verdad en el release de #30: un 500 transitorio del sumdb que el
#    fetch directo esquivó).
bump_module() {
  local module_dir=$1
  echo "📦 Bumpeando $module_dir/go.mod a $MODULE_PATH@$VERSION..."
  if ! (cd "$module_dir" && GOWORK=off go get "$MODULE_PATH@$VERSION" 2>/tmp/bump-core-err.$$); then
    echo "  camino normal falló, reintentando con GOPROXY=direct GOSUMDB=off..."
    cat /tmp/bump-core-err.$$ >&2
    (cd "$module_dir" && GOWORK=off GOPROXY=direct GOSUMDB=off go get "$MODULE_PATH@$VERSION")
  fi
  rm -f /tmp/bump-core-err.$$
  (cd "$module_dir" && GOWORK=off go mod tidy)
}

bump_module slidelang
bump_module doclang

# 7. Verificar que ambos CLIs compilan con GOWORK=off — la resolución real
#    de CI/go install, no la que go.work enmascara en desarrollo local.
echo "🔎 Verificando GOWORK=off go build en slidelang y doclang..."
(cd slidelang && GOWORK=off go build ./...)
(cd doclang && GOWORK=off go build ./...)
echo "✅ Ambos CLIs compilan contra $MODULE_PATH@$VERSION."

# 8. Opcional: rama + PR con el bump, si `gh` está disponible. Sin `gh`, el
#    diff de go.mod/go.sum queda en el working tree para que quien corrió el
#    script lo commitee/PRee a mano — igual que release.sh degrada cuando
#    `gh` no está instalado.
if command -v gh >/dev/null 2>&1; then
  BRANCH="chore/bump-core-$VERSION"
  echo "🌿 Creando rama $BRANCH y PR con el bump..."
  git checkout -b "$BRANCH"
  git add slidelang/go.mod slidelang/go.sum doclang/go.mod doclang/go.sum
  git commit -m "chore: bump core to $VERSION in slidelang and doclang go.mod"
  git push -u origin "$BRANCH"
  gh pr create --base main --head "$BRANCH" \
    --title "chore: bump core to $VERSION in slidelang and doclang go.mod" \
    --body "Automated by scripts/bump-core.sh — bumps \`require go.ziradocs.com/core/v2\` to \`$VERSION\` in both CLIs. Verified with \`GOWORK=off go build ./...\` in both modules before opening this PR."
  echo "✅ PR abierta."
else
  echo "⚠️ 'gh' no está instalado — el bump de go.mod/go.sum quedó en el working tree."
  echo "Revisá el diff, commiteá en una rama nueva, y abrí la PR a mano:"
  echo "  git checkout -b chore/bump-core-$VERSION"
  echo "  git add slidelang/go.mod slidelang/go.sum doclang/go.mod doclang/go.sum"
  echo "  git commit -m 'chore: bump core to $VERSION in slidelang and doclang go.mod'"
fi
