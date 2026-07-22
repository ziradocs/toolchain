#!/usr/bin/env bash
set -e

VERSION=$1

if [[ -z "$VERSION" ]]; then
  echo "Error: Debes proveer una versión (ej. v2.0.7)"
  exit 1
fi

if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+.*$ ]]; then
  echo "Error: La versión debe seguir SemVer y empezar con 'v' (ej. v2.0.7)"
  exit 1
fi

# 1. Asegurar que estamos limpios
if [[ -n $(git status -s) ]]; then
  echo "Error: Tienes cambios sin commitear. Haz commit o stash primero."
  exit 1
fi

# 2. Verificar que el tag coincida con el path del módulo de Go
#    Si es v2 o superior, el go.mod debe terminar en /v2, /v3, etc.
MAJOR_VERSION=$(echo "$VERSION" | grep -o '^v[0-9]*')
if [[ "$MAJOR_VERSION" != "v1" && "$MAJOR_VERSION" != "v0" ]]; then
  MODULE_VER=$(grep "^module" core/go.mod | awk -F'/' '{print $NF}')
  if [[ "$MODULE_VER" != "$MAJOR_VERSION" ]]; then
    echo "🔥 Error Crítico de Go: Estás intentando lanzar $VERSION, pero core/go.mod no tiene el sufijo /$MAJOR_VERSION."
    echo "Si vas a sacar una versión mayor nueva, debes actualizar los go.mod y todos los imports."
    exit 1
  fi
fi

# 3. Validar que NO estemos empujando cambios de workflows junto con el tag
# (Esto fue lo que rompió los triggers en v2.0.4)
git fetch origin main -q
UNPUSHED_WORKFLOWS=$(git log origin/main..HEAD --name-only --oneline | grep ".github/workflows/" || true)
if [[ -n "$UNPUSHED_WORKFLOWS" ]]; then
  echo "⚠️ Error de GitHub Actions: Tienes commits locales sin empujar que modifican .github/workflows/"
  echo "Si empujas archivos de workflow junto con tags usando credenciales locales, GitHub bloquea silenciosamente el lanzamiento."
  echo ""
  echo "Solución: Haz un 'git push' normal primero, y luego vuelve a ejecutar este script."
  exit 1
fi

echo "🚀 Todo se ve bien. Creando tags para $VERSION..."
git tag "$VERSION"
git tag "core/$VERSION"
git tag "doclang/$VERSION"
git tag "slidelang/$VERSION"

# 4. Empujar el tag que dispara el release SOLO, en su propio push.
#    (Empíricamente: con `git push origin --tags` empujando los 4 tags de
#    golpe -core/, doclang/, slidelang/ y el global- al mismo commit, GitHub
#    no dispara `on: push: tags:` de forma confiable -pasó en v2.0.6 y
#    v2.1.1-. El único release histórico que sí disparó por push -v2.0.0- se
#    empujó como tag único, antes de que este script existiera. Empujar el
#    tag de release aparte evita depender de ese comportamiento no documentado.)
echo "☁️ Empujando $VERSION (el tag que dispara el release)..."
git push origin "refs/tags/$VERSION"

echo "☁️ Empujando los tags de submódulo (core/doclang/slidelang)..."
git push origin "refs/tags/core/$VERSION" "refs/tags/doclang/$VERSION" "refs/tags/slidelang/$VERSION"

# 5. Confirmar que el workflow realmente arrancó; si no, dispararlo a mano.
#    Requiere `gh` autenticado (mismo supuesto que el resto del repo).
if command -v gh >/dev/null 2>&1; then
  echo "🔎 Verificando que el workflow de release haya arrancado..."
  triggered=false
  for _ in $(seq 1 6); do
    sleep 5
    if gh run list --workflow=release.yml --event=push --limit 5 --json headBranch,createdAt \
        --jq ".[] | select(.headBranch == \"$VERSION\")" 2>/dev/null | grep -q .; then
      triggered=true
      break
    fi
  done
  if [[ "$triggered" == true ]]; then
    echo "✅ El workflow de release arrancó por el push del tag."
  else
    echo "⚠️ El push del tag no disparó el workflow (ver comentario arriba). Disparándolo a mano..."
    gh workflow run release.yml --ref "$VERSION"
    echo "✅ Release disparado manualmente para $VERSION."
  fi
else
  echo "⚠️ 'gh' no está instalado — no pude verificar ni disparar el workflow automáticamente."
  echo "Revisa https://github.com/ziradocs/toolchain/actions y, si no arrancó, corre:"
  echo "  gh workflow run release.yml --ref $VERSION"
fi
