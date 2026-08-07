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
#
# El script es seguro de correr desde CUALQUIER rama: todo lo que produce
# (la rama del bump, el tag, el `go mod tidy`, el build de verificación) se
# calcula sobre "$REF", nunca sobre el HEAD en que lo corriste. Eso es lo que
# hace útil al argumento `ref` — ver el paso 4 y docs/developer/releasing.md.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

VERSION=${1:-}
REF=${2:-origin/main}
BRANCH="chore/bump-core-${VERSION}"

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

# 3b. Rechazar si la rama del bump ya existe (local o en origin). Se chequea
#     ACÁ, junto al check del tag y antes de pushear nada: si el `git checkout
#     -b` del paso 4 fallara después del push del tag, el tag ya estaría
#     quemado (los tags de módulo Go no se reusan — ver CLAUDE.md sobre
#     core/v2.1.0) y no habría forma de reintentar con la misma versión.
if git rev-parse --verify --quiet "refs/heads/$BRANCH" >/dev/null; then
  echo "Error: la rama $BRANCH ya existe local. Borrala o elegí otra versión."
  exit 1
fi
if git ls-remote --exit-code --heads origin "$BRANCH" >/dev/null 2>&1; then
  echo "Error: la rama $BRANCH ya existe en origin. Elegí otra versión."
  exit 1
fi

TAG_COMMIT=$(git rev-parse "$REF")

# 4. Crear la rama del bump DESDE "$REF" — no desde HEAD.
#
#    Esto no es cosmético y no basta con hacerlo al final, junto al commit:
#    los pasos 6 y 7 (`go mod tidy` y `GOWORK=off go build ./...`) leen los
#    .go del working tree, así que corriendo desde otra rama el tidy derivaría
#    el set de `require` del árbol equivocado — un import que esa rama agrega
#    mete un require espurio, y uno que quita borra un require que "$REF" sí
#    necesita, sin que el build de verificación note ninguno de los dos. Por
#    eso la rama se corta ANTES del bump, no después.
#
#    Un `git checkout -b` pelado (lo que este script hacía antes) hereda lo
#    que HEAD tenga encima de "$REF". Eso produjo un incidente real el
#    2026-08-06: el operador creía estar en main —un `git checkout main`
#    previo había fallado en silencio porque main estaba checkouteado en otro
#    worktree— y la PR del bump arrastró dos commits de feature ajenos,
#    mergeando una feature entera a main bajo un commit "chore:".
#
#    Cortar la rama con `git checkout -b "$BRANCH" "$TAG_COMMIT"` es inmune a
#    esa falla: crea la rama DESDE ese commit sin checkoutear "$REF" mismo, o
#    sea que funciona igual aunque main esté tomado por otro worktree.
AHEAD=$(git log --oneline -n 10 "$REF"..HEAD 2>/dev/null || true)
if [[ -n "$AHEAD" ]]; then
  echo "ℹ️ HEAD tiene commits que $REF no tiene. NO van a entrar en $BRANCH:"
  echo "$AHEAD" | sed 's/^/     /'
  echo "   (si alguno de estos debía publicarse en core/$VERSION, cancelá ahora"
  echo "    y mergealo a $REF primero — este script no mergea nada.)"
fi

echo "🌿 Creando rama $BRANCH desde $REF ($(git rev-parse --short "$TAG_COMMIT"))..."
git checkout -b "$BRANCH" "$TAG_COMMIT"
echo "   (a partir de acá quedás parado en $BRANCH, salga bien o falle a media corrida.)"

echo "🚀 Taggeando core/$VERSION en $REF ($TAG_COMMIT)..."

# 5. Crear y pushear SOLO core/vX.Y.Z — nunca un tag pelado vX.Y.Z.
#    .github/workflows/release.yml dispara con "v[0-9]*.[0-9]*.[0-9]*" Y con
#    un "v*" suelto (ver el comentario en ese archivo sobre por qué el "v*"
#    ahí es un riesgo real, no solo teórico); "core/vX.Y.Z" no empieza con
#    "v" así que ninguno de los dos patrones matchea — confirmado, esto NO
#    dispara goreleaser.
git tag -a "core/$VERSION" "$TAG_COMMIT" -m "core/$VERSION"
git push origin "refs/tags/core/$VERSION"

# 6. Calentar proxy.golang.org + sum.golang.org antes de intentar `go get`.
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
  echo "⚠️ El proxy/sumdb no confirmó $VERSION tras 12 intentos (60s) — se sigue igual; el paso 7 tiene su propio fallback."
fi

# 7. Bumpear el require en slidelang/go.mod y doclang/go.mod, vía GOWORK=off
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

# 8. Verificar que ambos CLIs compilan con GOWORK=off — la resolución real
#    de CI/go install, no la que go.work enmascara en desarrollo local.
echo "🔎 Verificando GOWORK=off go build en slidelang y doclang..."
(cd slidelang && GOWORK=off go build ./...)
(cd doclang && GOWORK=off go build ./...)
echo "✅ Ambos CLIs compilan contra $MODULE_PATH@$VERSION."

# 9. Opcional: commit + PR con el bump, si `gh` está disponible. La rama ya
#    existe desde el paso 4 (y ya estás parado en ella); acá solo se commitea
#    y se PRea. Sin `gh`, el diff de go.mod/go.sum queda sin commitear sobre
#    esa rama para que quien corrió el script lo commitee/PRee a mano — igual
#    que release.sh degrada cuando `gh` no está instalado.
if command -v gh >/dev/null 2>&1; then
  echo "📝 Commiteando el bump en $BRANCH y abriendo la PR..."
  git add slidelang/go.mod slidelang/go.sum doclang/go.mod doclang/go.sum
  # GOWORK=off también en el commit: .githooks/pre-commit corre validaciones
  # de Go, y un go.work heredado (p.ej. desde un worktree bajo .claude/) las
  # rompe con un error que no tiene nada que ver con este bump.
  GOWORK=off git commit -m "chore: bump core to $VERSION in slidelang and doclang go.mod"
  git push -u origin "$BRANCH"
  gh pr create --base main --head "$BRANCH" \
    --title "chore: bump core to $VERSION in slidelang and doclang go.mod" \
    --body "Automated by scripts/bump-core.sh — bumps \`require go.ziradocs.com/core/v2\` to \`$VERSION\` in both CLIs. Branch cut from \`$REF\` ($(git rev-parse --short "$TAG_COMMIT")), so the diff is the bump and nothing else. Verified with \`GOWORK=off go build ./...\` in both modules before opening this PR."
  echo "✅ PR abierta."
else
  echo "⚠️ 'gh' no está instalado — el bump de go.mod/go.sum quedó sin commitear."
  echo "Ya estás parado en $BRANCH (cortada desde $REF). Revisá el diff, commiteá y abrí la PR a mano:"
  echo "  git add slidelang/go.mod slidelang/go.sum doclang/go.mod doclang/go.sum"
  echo "  GOWORK=off git commit -m 'chore: bump core to $VERSION in slidelang and doclang go.mod'"
  echo "  git push -u origin $BRANCH"
fi
