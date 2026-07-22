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

echo "☁️ Empujando tags a GitHub..."
git push origin --tags

echo "✅ ¡Tags publicados! El GitHub Action de release debería arrancar en unos segundos."
