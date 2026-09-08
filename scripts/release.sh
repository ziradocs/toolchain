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

# 4. Verificar que ambos CLIs compilan contra el core PUBLICADO que tienen
#    pineado en su `require`. goreleaser corre en CI, sin go.work, así que
#    resuelve ese pin desde el proxy; un go.work local enmascara el problema
#    por completo (siempre usa el core del árbol, esté taggeado o no). Sin
#    este check, un pin desactualizado se descubre cuando el release ya falló
#    a medias — con los 4 tags pusheados y por lo tanto quemados.
#    Es el mismo check que scripts/bump-core.sh corre en su paso 8; acá es el
#    cinturón de seguridad para el caso en que el bump nunca se hizo.
echo "🔎 Verificando GOWORK=off go build en slidelang y doclang..."
(cd slidelang && GOWORK=off go build ./...)
(cd doclang && GOWORK=off go build ./...)
echo "✅ Ambos CLIs compilan contra el core publicado que tienen pineado."

# 4b. `core/$VERSION` puede existir YA, y es el caso normal: cuando el release
#     lleva un cambio de core, `scripts/bump-core.sh $VERSION` lo cortó y lo
#     empujó antes, porque los CLIs necesitaban poder pinearlo.
#
#     Este script tageaba los cuatro a ciegas. Con `set -e`, el `git tag
#     core/$VERSION` fallaba con "already exists" DESPUÉS de haber creado el
#     `$VERSION` pelado local, así que abortaba a mitad y dejaba basura. En la
#     práctica eso quemaba el número: el release salía con el siguiente libre
#     —pasó con v2.32.3, que quedó saltada— y cada core-bump costaba una
#     versión de producto.
#
#     Reusarlo es correcto siempre que apunte al MISMO commit que se está
#     liberando; si apunta a otro, el release publicaría binarios construidos
#     contra un core distinto del que dice su tag, y eso sí hay que frenarlo.
CORE_TAG="core/$VERSION"
REUSE_CORE_TAG=false
if git rev-parse -q --verify "refs/tags/$CORE_TAG" >/dev/null; then
  REUSE_CORE_TAG=true
elif git ls-remote --exit-code --tags origin "refs/tags/$CORE_TAG" >/dev/null 2>&1; then
  git fetch -q origin "refs/tags/$CORE_TAG:refs/tags/$CORE_TAG"
  REUSE_CORE_TAG=true
fi

if [[ "$REUSE_CORE_TAG" == true ]]; then
  CORE_TAG_COMMIT=$(git rev-list -n 1 "$CORE_TAG")
  HEAD_COMMIT=$(git rev-parse HEAD)
  if [[ "$CORE_TAG_COMMIT" != "$HEAD_COMMIT" ]]; then
    echo "🔥 $CORE_TAG ya existe pero apunta a $CORE_TAG_COMMIT, y estás liberando $HEAD_COMMIT."
    echo "   Publicar así daría binarios construidos contra un core distinto del que declara su tag."
    echo "   Revisá si el bump quedó sin mergear, o usá el siguiente número libre."
    exit 1
  fi
  echo "ℹ️ $CORE_TAG ya existe en este mismo commit (lo cortó bump-core.sh). Se reusa."
fi

echo "🚀 Todo se ve bien. Creando tags para $VERSION..."
git tag "$VERSION"
if [[ "$REUSE_CORE_TAG" == false ]]; then
  git tag "$CORE_TAG"
fi
git tag "doclang/$VERSION"
git tag "slidelang/$VERSION"

# 5. Empujar el tag que dispara el release SOLO, en su propio push.
#    (Empíricamente: con `git push origin --tags` empujando los 4 tags de
#    golpe -core/, doclang/, slidelang/ y el global- al mismo commit, GitHub
#    no dispara `on: push: tags:` de forma confiable -pasó en v2.0.6 y
#    v2.1.1-. El único release histórico que sí disparó por push -v2.0.0- se
#    empujó como tag único, antes de que este script existiera. Empujar el
#    tag de release aparte evita depender de ese comportamiento no documentado.)
echo "☁️ Empujando $VERSION (el tag que dispara el release)..."
git push origin "refs/tags/$VERSION"

echo "☁️ Empujando los tags de submódulo (core/doclang/slidelang)..."
SUBMODULE_TAGS=("refs/tags/doclang/$VERSION" "refs/tags/slidelang/$VERSION")
if [[ "$REUSE_CORE_TAG" == false ]]; then
  SUBMODULE_TAGS+=("refs/tags/$CORE_TAG")
fi
git push origin "${SUBMODULE_TAGS[@]}"

# 6. Confirmar que el workflow realmente arrancó; si no, dispararlo a mano.
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
