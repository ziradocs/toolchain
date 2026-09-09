#!/usr/bin/env bash
# Gate ejecutable de la decisión "reusar core/$VERSION" de scripts/release.sh.
#
# Por qué existe: esa lógica ya se rompió dos veces seguidas, y las dos veces la
# rotura era un ABORT —primero por colisión de tag, después por exigir que el
# tag apuntara a HEAD, cosa que en el flujo real nunca pasa— así que nada se
# ponía rojo hasta el día del release, con el número quemado. `bash -n` no ve
# nada de esto: la sintaxis siempre estuvo bien.
#
# Cómo: repos de git temporales con su propio `origin` bare, y un PATH armado a
# mano donde un `go` falso tapa al real (release.sh compila los dos CLIs) y
# donde `gh` NO está, para que el script tome su rama de "no pude verificar el
# workflow". No se toca la red ni el repo real. Cada escenario afirma tres
# cosas, porque con menos pasa un abort por la razón equivocada:
#
#   1. el código de salida;
#   2. un pedazo del mensaje que identifica ESA condición y no otra;
#   3. los tags que quedaron en el origin bare — que es lo único que distingue
#      "reusó el tag de core" de "lo volvió a crear".
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RELEASE_SH="$SCRIPT_DIR/release.sh"
VERSION="v2.99.0"
VERSION_VIEJA="v2.98.0"
CORE_TAG="core/$VERSION"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

REAL_GIT="$(command -v git)"

# El `go` falso: release.sh corre `GOWORK=off go build ./...` en los dos CLIs y
# acá no hay código Go que compilar. Va PRIMERO en el PATH para tapar al real.
mkdir -p "$TMP/stub"
printf '#!/usr/bin/env bash\nexit 0\n' > "$TMP/stub/go"
chmod +x "$TMP/stub/go"

# El `gh` falso. La primera versión de este harness NO lo traía, apostando a
# que `command -v gh` fallara por un PATH recortado: pasaba en macOS (gh vive en
# /opt/homebrew/bin) y fallaba en el runner de Ubuntu, donde gh está en
# /usr/bin, o sea DENTRO del PATH mínimo. Depender de la ausencia de una
# herramienta no es hermético; el stub sí.
#
# Simula el caso normal —el push del tag disparó el workflow— para que el paso 6
# de release.sh no espere seis rondas ni intente dispararlo a mano. Cualquier
# otro subcomando falla ruidoso: es un cable trampa, este sandbox no le habla a
# GitHub.
cat > "$TMP/stub/gh" <<'EOF'
#!/usr/bin/env bash
case "$1 $2" in
  "run list")      echo '{"headBranch":"v2.99.0","createdAt":"2026-01-01T00:00:00Z"}'; exit 0 ;;
  "workflow run")  exit 0 ;;
esac
echo "stub de gh: subcomando no esperado en el sandbox: $*" >&2
exit 1
EOF
chmod +x "$TMP/stub/gh"

BASE_PATH="$TMP/stub:/usr/bin:/bin"

# El wrapper de git que hace fallar SOLO `ls-remote`, para el escenario de
# fail-closed. Es el único lugar donde se envuelve git, y hace falta: en una
# caída de red de verdad el `git fetch origin main` del paso 3 aborta antes y
# nunca se llega al helper, así que la única forma de ejercitar esa rama es
# fallar justo ahí. Todo lo demás pasa al git real.
mkdir -p "$TMP/stub-lsremote"
cat > "$TMP/stub-lsremote/git" <<EOF
#!/usr/bin/env bash
for a in "\$@"; do
  if [[ "\$a" == "ls-remote" ]]; then
    echo "fatal: could not read from remote repository (simulado)" >&2
    exit 128
  fi
done
exec "$REAL_GIT" "\$@"
EOF
chmod +x "$TMP/stub-lsremote/git"

fallos=0

# ---------------------------------------------------------------- utilidades

nuevo_repo() { # $1 = nombre → deja $TMP/$1/{origin.git,repo}
  local d="$TMP/$1"
  mkdir -p "$d"
  git init -q --bare "$d/origin.git"
  git init -q -b main "$d/repo"
  git -C "$d/repo" config user.email "test@example.invalid"
  git -C "$d/repo" config user.name "Test"
  git -C "$d/repo" remote add origin "$d/origin.git"

  mkdir -p "$d/repo/core" "$d/repo/slidelang" "$d/repo/doclang"
  printf 'module go.ziradocs.com/core/v2\n\ngo 1.26.5\n' > "$d/repo/core/go.mod"
  printf 'contenido inicial\n' > "$d/repo/core/parser.go"
  pin_core "$d/repo" "$VERSION_VIEJA"
  git -C "$d/repo" add -A
  git -C "$d/repo" commit -q -m "inicial"
}

pin_core() { # $1 = repo, $2 = versión de core que pinean los dos CLIs
  local r=$1 v=$2 m
  for m in slidelang doclang; do
    printf 'module go.ziradocs.com/%s/v2\n\ngo 1.26.5\n\nrequire (\n\tgo.ziradocs.com/core/v2 %s\n)\n' "$m" "$v" > "$r/$m/go.mod"
  done
}

commit_en() { # $1 = repo, $2 = archivo, $3 = contenido, $4 = mensaje
  printf '%s\n' "$3" > "$1/$2"
  git -C "$1" add -A
  git -C "$1" commit -q -m "$4"
}

correr_release() { # $1 = repo, $2 = prefijo extra de PATH (opcional)
  local repo=$1 extra=${2:-} url rc=0
  url=$(git -C "$repo" remote get-url origin)
  case "$url" in
    "$TMP"/*) ;;
    *) echo "🔥 ABORTANDO: el origin del sandbox apunta a '$url', fuera de $TMP." >&2
       echo "   Este script corre 'git push'; no se ejecuta contra un remoto real." >&2
       exit 1 ;;
  esac
  set +e
  ( cd "$repo" && PATH="${extra:+$extra:}$BASE_PATH" bash "$RELEASE_SH" "$VERSION" ) \
    > "$TMP/salida.txt" 2>&1
  rc=$?
  set -e
  return $rc
}

tags_origin() { git -C "$1/origin.git" tag -l | sort | tr '\n' ' '; }
sha_tag_origin() { git -C "$1/origin.git" rev-list -n 1 "$2"; }

# reportar TERMINA el escenario, con código 2. No incrementa el contador: corre
# dentro del subshell de correr_escenario, así que una variable del padre no se
# vería. El 2 es lo que distingue "falló y ya lo expliqué" de cualquier otro
# código, que es un fallo de PREPARACIÓN sin reportar y el padre lo dice.
reportar() { # $1 = escenario, $2 = qué falló
  echo "❌ $1: $2"
  echo "   ---- salida de release.sh ----"
  sed 's/^/   /' "$TMP/salida.txt"
  exit 2
}

verificar() { # $1 = escenario, $2 = rc obtenido, $3 = rc esperado, $4 = substring esperado
  local esc=$1 rc=$2 rc_esperado=$3 msg=$4
  if [[ "$rc" != "$rc_esperado" ]]; then
    reportar "$esc" "salió $rc, se esperaba $rc_esperado"
    return 1
  fi
  if [[ -n "$msg" ]] && ! grep -qF "$msg" "$TMP/salida.txt"; then
    reportar "$esc" "salió $rc pero por otra razón: no dice '$msg'"
    return 1
  fi
  return 0
}

# ------------------------------------------------------------- escenarios

# 1. El flujo real, que es el que la versión anterior de este guard rompía:
#    bump-core.sh cortó core/$VERSION sobre el origin/main del momento y DESPUÉS
#    se mergearon el PR del bump y los de los CLIs. Para cuando se libera, el
#    tag es un ancestro varios commits atrás — nunca HEAD.
escenario_flujo_real() {
  local esc="flujo-real (tag ancestro, core/ intacto, pins al día)" d="$TMP/flujo-real"
  nuevo_repo flujo-real
  commit_en "$d/repo" core/parser.go "el core que se va a liberar" "core: cambio"
  git -C "$d/repo" tag -a -m "core $VERSION" "$CORE_TAG"
  pin_core "$d/repo" "$VERSION"
  git -C "$d/repo" add -A && git -C "$d/repo" commit -q -m "bump core a $VERSION"
  commit_en "$d/repo" slidelang/main.go "cambio de CLI posterior al tag" "slidelang: cambio"
  git -C "$d/repo" push -q origin main
  git -C "$d/repo" push -q origin "refs/tags/$CORE_TAG"
  local sha_antes; sha_antes=$(sha_tag_origin "$d" "$CORE_TAG")

  local rc=0; correr_release "$d/repo" || rc=$?
  verificar "$esc" "$rc" 0 "Se reusa." || return
  if [[ "$(sha_tag_origin "$d" "$CORE_TAG")" != "$sha_antes" ]]; then
    reportar "$esc" "$CORE_TAG se movió en origin: se recreó en vez de reusarse"; return
  fi
  local esperado="core/$VERSION doclang/$VERSION slidelang/$VERSION $VERSION "
  if [[ "$(tags_origin "$d")" != "$esperado" ]]; then
    reportar "$esc" "tags en origin: '$(tags_origin "$d")', se esperaba '$esperado'"; return
  fi
  echo "✅ $esc"
}

# 2. Un release sin cambios de core: nadie cortó el tag antes, así que este
#    script lo crea junto con los otros tres. Sin esta fila, "reusar siempre"
#    pasaría el resto de la suite.
escenario_sin_tag() {
  local esc="sin-tag (nadie cortó core antes; se crea)" d="$TMP/sin-tag"
  nuevo_repo sin-tag
  # Los dos go.mod siguen pineando VERSION_VIEJA, y tiene que ser así: en un
  # release sin cambios de core NADIE cortó core/$VERSION, así que un
  # `GOWORK=off go build` real no podría resolver ese pin. La primera versión
  # de este fixture pineaba $VERSION y solo pasaba porque `go` está stubbeado:
  # modelaba un estado imposible. Acá el commit posterior toca un CLI, que es
  # lo que un release así lleva.
  commit_en "$d/repo" slidelang/main.go "cambio de CLI sin tocar core" "slidelang: cambio"
  git -C "$d/repo" push -q origin main
  local head; head=$(git -C "$d/repo" rev-parse HEAD)

  local rc=0; correr_release "$d/repo" || rc=$?
  verificar "$esc" "$rc" 0 "" || return
  local esperado="core/$VERSION doclang/$VERSION slidelang/$VERSION $VERSION "
  if [[ "$(tags_origin "$d")" != "$esperado" ]]; then
    reportar "$esc" "tags en origin: '$(tags_origin "$d")', se esperaba '$esperado'"; return
  fi
  if [[ "$(sha_tag_origin "$d" "$CORE_TAG")" != "$head" ]]; then
    reportar "$esc" "$CORE_TAG no quedó en HEAD"; return
  fi
  echo "✅ $esc"
}

# 3. El tag existe LOCAL y no en origin. goreleaser resuelve el core desde el
#    proxy, que lee origin: un tag que no está allá no existe para el build.
escenario_solo_local() {
  local esc="solo-local (tag que no está en origin)" d="$TMP/solo-local"
  nuevo_repo solo-local
  commit_en "$d/repo" core/parser.go "core nuevo" "core: cambio"
  git -C "$d/repo" tag -a -m "core $VERSION" "$CORE_TAG"
  pin_core "$d/repo" "$VERSION"
  git -C "$d/repo" add -A && git -C "$d/repo" commit -q -m "bump core a $VERSION"
  git -C "$d/repo" push -q origin main   # el tag NO se empuja

  local rc=0; correr_release "$d/repo" || rc=$?
  verificar "$esc" "$rc" 1 "existe LOCAL pero no en origin" || return
  if [[ -n "$(tags_origin "$d")" ]]; then
    reportar "$esc" "abortó pero dejó tags en origin: '$(tags_origin "$d")'"; return
  fi
  echo "✅ $esc"
}

# 4. El tag salió de otra línea de commits: el core publicado no es el de este
#    release, aunque el número coincida.
escenario_otra_linea() {
  local esc="otra-linea (tag que no es ancestro de HEAD)" d="$TMP/otra-linea"
  nuevo_repo otra-linea
  git -C "$d/repo" checkout -q -b desvio
  commit_en "$d/repo" core/parser.go "core de otra rama" "core: rama paralela"
  git -C "$d/repo" tag -a -m "core $VERSION" "$CORE_TAG"
  git -C "$d/repo" checkout -q main
  pin_core "$d/repo" "$VERSION"
  git -C "$d/repo" add -A && git -C "$d/repo" commit -q -m "bump core a $VERSION"
  git -C "$d/repo" push -q origin main
  git -C "$d/repo" push -q origin "refs/tags/$CORE_TAG"

  local rc=0; correr_release "$d/repo" || rc=$?
  verificar "$esc" "$rc" 1 "NO es ancestro de HEAD" || return
  if [[ "$(tags_origin "$d")" != "core/$VERSION " ]]; then
    reportar "$esc" "abortó pero movió tags: '$(tags_origin "$d")'"; return
  fi
  echo "✅ $esc"
}

# 5. `core/` cambió entre el tag y HEAD: lo taggeado no es lo que hay.
escenario_core_cambio() {
  local esc="core-cambio (core/ modificado después del tag)" d="$TMP/core-cambio"
  nuevo_repo core-cambio
  commit_en "$d/repo" core/parser.go "core al momento del tag" "core: cambio"
  git -C "$d/repo" tag -a -m "core $VERSION" "$CORE_TAG"
  pin_core "$d/repo" "$VERSION"
  git -C "$d/repo" add -A && git -C "$d/repo" commit -q -m "bump core a $VERSION"
  commit_en "$d/repo" core/parser.go "core CAMBIADO después del tag" "core: cambio tardío"
  git -C "$d/repo" push -q origin main
  git -C "$d/repo" push -q origin "refs/tags/$CORE_TAG"

  local rc=0; correr_release "$d/repo" || rc=$?
  verificar "$esc" "$rc" 1 "cambió entre" || return
  if [[ "$(tags_origin "$d")" != "core/$VERSION " ]]; then
    reportar "$esc" "abortó pero movió tags: '$(tags_origin "$d")'"; return
  fi
  echo "✅ $esc"
}

# 6. Falta mergear el PR del bump: los go.mod pinean otra versión, que es la que
#    goreleaser va a resolver del proxy.
escenario_go_mod_viejo() {
  local esc="go-mod-viejo (los CLIs pinean otra versión)" d="$TMP/go-mod-viejo"
  nuevo_repo go-mod-viejo
  commit_en "$d/repo" core/parser.go "core nuevo" "core: cambio"
  git -C "$d/repo" tag -a -m "core $VERSION" "$CORE_TAG"
  commit_en "$d/repo" slidelang/main.go "cambio de CLI" "slidelang: cambio"
  git -C "$d/repo" push -q origin main
  git -C "$d/repo" push -q origin "refs/tags/$CORE_TAG"

  local rc=0; correr_release "$d/repo" || rc=$?
  verificar "$esc" "$rc" 1 "no pinea core $VERSION" || return
  if [[ "$(tags_origin "$d")" != "core/$VERSION " ]]; then
    reportar "$esc" "abortó pero movió tags: '$(tags_origin "$d")'"; return
  fi
  echo "✅ $esc"
}

# 7. Fail-closed: un `ls-remote` que sale distinto de 0 NO se lee como "el tag
#    no existe". Si se leyera así, el script crearía un core/$VERSION nuevo
#    encima de uno que quizá ya está publicado.
escenario_ls_remote_falla() {
  local esc="ls-remote-falla (falla cerrado, no como 'no existe')" d="$TMP/ls-remote"
  nuevo_repo ls-remote
  pin_core "$d/repo" "$VERSION"
  git -C "$d/repo" add -A && git -C "$d/repo" commit -q -m "bump core a $VERSION"
  git -C "$d/repo" push -q origin main

  local rc=0; correr_release "$d/repo" "$TMP/stub-lsremote" || rc=$?
  verificar "$esc" "$rc" 1 "git ls-remote salió" || return
  if [[ -n "$(tags_origin "$d")" ]]; then
    reportar "$esc" "abortó pero dejó tags en origin: '$(tags_origin "$d")'"; return
  fi
  echo "✅ $esc"
}

# Cada escenario corre en su propio subshell y el padre captura el código.
#
# La versión anterior era `escenario_x || true`, y eso APAGA errexit dentro de
# toda la función: en bash, `set -e` se ignora en cualquier comando que sea
# operando de un `&&`/`||`, y la supresión alcanza al cuerpo entero. Medido: un
# `git commit` de preparación que fallaba con "nothing to commit" no cortaba
# nada y el escenario llegaba a imprimir ✅. Un falso verde en el harness es
# peor que no tener harness.
#
# El subshell no puede ir en un `||` por la misma razón, así que el padre apaga
# errexit explícitamente alrededor de la llamada y lo vuelve a prender. Como el
# contador vive en el padre, `reportar` no puede tocarlo: sale con 2 y el padre
# cuenta.
correr_escenario() { # $1 = nombre de la función del escenario
  local rc=0
  set +e
  ( set -euo pipefail; "$1" )
  rc=$?
  set -e
  case "$rc" in
    0) ;;
    2) fallos=$((fallos + 1)) ;;  # ya lo explicó reportar
    *) echo "❌ $1: el escenario cortó con código $rc ANTES de afirmar nada."
       echo "   Eso es un fallo de preparación del propio harness, no del script bajo prueba."
       fallos=$((fallos + 1)) ;;
  esac
}

correr_escenario escenario_flujo_real
correr_escenario escenario_sin_tag
correr_escenario escenario_solo_local
correr_escenario escenario_otra_linea
correr_escenario escenario_core_cambio
correr_escenario escenario_go_mod_viejo
correr_escenario escenario_ls_remote_falla

echo
if (( fallos > 0 )); then
  echo "🔥 $fallos escenario(s) fallaron."
  exit 1
fi
echo "✅ Los 7 escenarios pasaron."
