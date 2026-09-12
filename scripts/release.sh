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

# 4b. `core/$VERSION` puede existir YA, y es el caso NORMAL: cuando el release
#     lleva un cambio de core, `scripts/bump-core.sh $VERSION` lo cortó antes,
#     porque los CLIs necesitaban poder pinearlo.
#
#     Este script tageaba los cuatro a ciegas. Con `set -e`, el `git tag
#     core/$VERSION` fallaba con "already exists" DESPUÉS de haber creado el
#     `$VERSION` pelado local, así que abortaba a mitad. En la práctica eso
#     quemaba el número: el release salía con el siguiente libre —pasó con
#     v2.32.3, que quedó saltada— y cada bump de core costaba una versión.
#
#     Qué NO se puede exigir: que el tag apunte a HEAD. En el flujo real nunca
#     lo hace. El tag se corta sobre el `origin/main` del momento y DESPUÉS
#     mergean el PR del bump y los PRs de los CLIs, así que para cuando se
#     libera, el tag es un ancestro varios commits atrás. Una primera versión
#     de este guard exigía el mismo commit y abortaba siempre.
#
#     Lo que sí importa es que el core PUBLICADO bajo ese tag sea el que se
#     está liberando, y eso son tres condiciones verificables:
#       (a) el commit del tag es ancestro de HEAD — el tag salió de esta línea;
#       (b) `core/` no cambió entre el tag y HEAD — lo taggeado es lo que hay;
#       (c) los dos go.mod pinean exactamente ese `core/$VERSION` — que es lo
#           que goreleaser va a resolver del proxy.
remote_ref_existe() {
  local tipo=$1 patron=$2
  local salida rc=0
  salida=$(git ls-remote "$tipo" origin "$patron") || rc=$?
  if (( rc != 0 )); then
    echo "Error: no pude consultar '$patron' en origin — git ls-remote salió $rc." >&2
    echo "Eso es una falla de red/credenciales, NO un 'no existe'. Abortando antes de taggear nada." >&2
    exit 1
  fi
  [[ -n "$salida" ]]
}

CORE_TAG="core/$VERSION"
REUSE_CORE_TAG=false

#     El REMOTO es la autoridad, siempre, aunque haya un tag local con ese
#     nombre: un local que apunte a otro commit que el de origin es justo el
#     estado peligroso, y consultar solo local no lo vería. El helper falla
#     CERRADO —un ls-remote que sale distinto de 0 aborta en vez de leerse
#     como "no existe"—, igual que en bump-core.sh, de donde está copiado.
if remote_ref_existe --tags "refs/tags/$CORE_TAG"; then
  REUSE_CORE_TAG=true
  git fetch -q --force origin "refs/tags/$CORE_TAG:refs/tags/$CORE_TAG"
elif git rev-parse -q --verify "refs/tags/$CORE_TAG" >/dev/null; then
  echo "🔥 $CORE_TAG existe LOCAL pero no en origin."
  echo "   goreleaser resuelve el core desde el proxy, que lee origin: un tag que no está allá"
  echo "   no existe para el build. Empujalo (o borralo) antes de liberar."
  exit 1
fi

if [[ "$REUSE_CORE_TAG" == true ]]; then
  CORE_TAG_COMMIT=$(git rev-list -n 1 "$CORE_TAG")
  if ! git merge-base --is-ancestor "$CORE_TAG_COMMIT" HEAD; then
    echo "🔥 $CORE_TAG apunta a $(git rev-parse --short "$CORE_TAG_COMMIT"), que NO es ancestro de HEAD."
    echo "   Ese tag salió de otra línea de commits; el core publicado no es el de este release."
    exit 1
  fi
  if ! git diff --quiet "$CORE_TAG_COMMIT" HEAD -- core/; then
    echo "🔥 core/ cambió entre $CORE_TAG y HEAD:"
    git diff --stat "$CORE_TAG_COMMIT" HEAD -- core/ | sed 's/^/     /'
    echo "   El tag no describe el core que se está liberando. Cortá un core/ nuevo con bump-core.sh."
    exit 1
  fi
  #     La comparación es EXACTA, con el campo extraído, y no un grep con
  #     `$VERSION\b`. Esa versión anterior daba falsos positivos porque en una
  #     ERE el `-` es frontera de palabra: `v2.32.5\b` matchea
  #     `v2.32.5-0.2026…-abc123`. Y ese no es un caso rebuscado — es el
  #     pseudo-version que Go escribe sola. Con `core/v2.32.4` ya cortado,
  #     cualquier `go get go.ziradocs.com/core/v2@main` sobre un commit
  #     posterior pinea `v2.32.5-0.<ts>-<sha>`: exactamente la versión que se
  #     está por liberar, seguida de un guion. Medido: el script decía "ambos
  #     go.mod lo pinean. Se reusa." y liberaba, con goreleaser resolviendo un
  #     commit SIN tag. Un `-rc1` a mano reproduce igual.
  #
  #     De paso desaparece la otra mitad del problema: `$VERSION` entraba sin
  #     escapar a la ERE, así que sus puntos eran comodines.
  #
  #     El awk lee las dos formas del require —la de bloque, donde el módulo es
  #     el primer campo, y la de una línea, donde lo precede `require`— y
  #     saltea cualquier línea con `=>`, que es un replace y no un pin.
  for m in slidelang doclang; do
    pin=$(awk '/=>/ { next }
               { for (i = 1; i < NF; i++)
                   if ($i ~ /^go\.ziradocs\.com\/core\/v[0-9]+$/) { print $(i+1); exit } }' "$m/go.mod")
    if [[ "$pin" != "$VERSION" ]]; then
      echo "🔥 $m/go.mod no pinea core $VERSION, pinea: ${pin:-(ninguno)}"
      echo "   Falta mergear el PR del bump antes de liberar."
      exit 1
    fi
  done
  echo "ℹ️ $CORE_TAG ya existe (lo cortó bump-core.sh) en $(git rev-parse --short "$CORE_TAG_COMMIT"):"
  echo "   ancestro de HEAD, core/ sin cambios desde entonces, y ambos go.mod lo pinean. Se reusa."
fi

# 4c. Los otros tres tags —`$VERSION`, `doclang/` y `slidelang/`— se creaban a
#     CIEGAS, que es la misma falla que 4b arregla para core/ y por la que este
#     script existe. Con `set -e`, cualquier segunda corrida moría en
#     `git tag "$VERSION"` con "already exists", DESPUÉS de pasar todo el bloque
#     de reuso y sin haber hecho nada. Y una segunda corrida no es rara: basta
#     que la primera muera en el push (red) o en el paso 6 (`gh`), con los tags
#     ya creados.
#
#     A diferencia de core/, estos tres los corta ESTE script sobre HEAD, así
#     que la condición de reuso es más simple: si ya existe, tiene que apuntar a
#     HEAD. Si apunta a otro commit, ese número ya se liberó desde otra línea y
#     el release se detiene — reusarlo publicaría un binario que no es el que
#     dice ser.
#
#     El remoto manda, igual que en 4b y por lo mismo: un tag local que apunte a
#     otro commit que el de origin es el estado peligroso, y consultar solo
#     local no lo vería. Se reusa el helper fail-closed.
HEAD_COMMIT=$(git rev-parse HEAD)
TAG_NECESITA_PUSH=false

resolver_tag_en_head() { # $1 = nombre del tag
  local tag=$1 sha
  TAG_NECESITA_PUSH=false

  if remote_ref_existe --tags "refs/tags/$tag"; then
    git fetch -q --force origin "refs/tags/$tag:refs/tags/$tag"
    sha=$(git rev-list -n 1 "refs/tags/$tag")
    if [[ "$sha" != "$HEAD_COMMIT" ]]; then
      echo "🔥 $tag ya existe en origin, en $(git rev-parse --short "$sha"), que NO es HEAD."
      echo "   Ese número ya se liberó desde otro commit. Elegí el siguiente."
      exit 1
    fi
    echo "ℹ️ $tag ya está en origin apuntando a HEAD (corrida previa). No se re-empuja."
    return
  fi

  if git rev-parse -q --verify "refs/tags/$tag" >/dev/null; then
    sha=$(git rev-list -n 1 "refs/tags/$tag")
    if [[ "$sha" != "$HEAD_COMMIT" ]]; then
      echo "🔥 $tag existe LOCAL en $(git rev-parse --short "$sha"), que NO es HEAD."
      echo "   Empujarlo así publicaría otro commit. Borralo o movelo antes de liberar."
      exit 1
    fi
    echo "ℹ️ $tag ya existía local en HEAD (corrida previa que no llegó a empujar). Se reusa."
  else
    git tag "$tag"
  fi
  TAG_NECESITA_PUSH=true
}

echo "🚀 Todo se ve bien. Resolviendo tags para $VERSION..."
resolver_tag_en_head "$VERSION"
PUSH_VERSION=$TAG_NECESITA_PUSH
if [[ "$REUSE_CORE_TAG" == false ]]; then
  git tag "$CORE_TAG"
fi
SUBMODULE_TAGS=()
for sub in "doclang/$VERSION" "slidelang/$VERSION"; do
  resolver_tag_en_head "$sub"
  if [[ "$TAG_NECESITA_PUSH" == true ]]; then
    SUBMODULE_TAGS+=("refs/tags/$sub")
  fi
done
if [[ "$REUSE_CORE_TAG" == false ]]; then
  SUBMODULE_TAGS+=("refs/tags/$CORE_TAG")
fi

# 5. Empujar el tag que dispara el release SOLO, en su propio push.
#    (Empíricamente: con `git push origin --tags` empujando los 4 tags de
#    golpe -core/, doclang/, slidelang/ y el global- al mismo commit, GitHub
#    no dispara `on: push: tags:` de forma confiable -pasó en v2.0.6 y
#    v2.1.1-. El único release histórico que sí disparó por push -v2.0.0- se
#    empujó como tag único, antes de que este script existiera. Empujar el
#    tag de release aparte evita depender de ese comportamiento no documentado.)
if [[ "$PUSH_VERSION" == true ]]; then
  echo "☁️ Empujando $VERSION (el tag que dispara el release)..."
  git push origin "refs/tags/$VERSION"
else
  echo "⏭️ $VERSION ya estaba en origin; no hay push que disparar. El paso 6 decide."
fi

# El guard de lista vacía NO es defensivo: en una re-corrida completa los tres
# tags ya están en origin, SUBMODULE_TAGS queda vacío y `git push origin
# "${SUBMODULE_TAGS[@]}"` se expande a `git push origin` pelado — que empuja la
# RAMA actual. Este script corre desde main y con el árbol limpio, así que sería
# un push silencioso y no una falla visible.
if (( ${#SUBMODULE_TAGS[@]} > 0 )); then
  echo "☁️ Empujando los tags de submódulo (core/doclang/slidelang)..."
  git push origin "${SUBMODULE_TAGS[@]}"
else
  echo "⏭️ Los tags de submódulo ya estaban en origin apuntando a HEAD."
fi

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
