#!/usr/bin/env bash
#
# Reports every direct Go module, Go toolchain patch, and pinned GitHub Action
# release that a newer version exists for. Exit codes:
#
#   0  everything current
#   1  at least one item outdated
#   2  a section could not run (no network, missing tool) and
#      DEPS_CHECK_OFFLINE_OK is unset — a gate must not pass unverified
#
# GO and GH are overridable so tests can substitute stubs.

set -euo pipefail

GO="${GO:-go}"
GH="${GH:-gh}"

root=""
while [ $# -gt 0 ]; do
  case "$1" in
    --root)
      [ $# -ge 2 ] || { echo "--root needs a directory" >&2; exit 64; }
      root="$2"
      shift 2
      ;;
    -h|--help)
      echo "usage: ${0##*/} [--root <dir>]"
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 64
      ;;
  esac
done
if [ -n "$root" ]; then
  cd "$root"
fi

outdated=0
unverified=0
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

skip_section() {
  if [ "${DEPS_CHECK_OFFLINE_OK:-}" = "1" ]; then
    return 0
  fi
  echo "SKIP $1: $2"
  unverified=1
}

# True when $1 sorts strictly before $2 under version ordering.
version_lt() {
  [ "$1" != "$2" ] &&
    [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -1)" = "$1" ]
}

first_line() {
  local msg
  msg="$(tr '\n' ' ' <"$1" | cut -c1-200)"
  printf '%s\n' "${msg:-command failed with no diagnostic}"
}

# --- 1. direct Go modules --------------------------------------------------
# -u fills Update only when a newer version exists; Main and Indirect drop the
# module itself and the transitive graph, whose upgrades follow from the direct
# ones.
mod_tmpl='{{if and (not .Main) (not .Indirect) .Update}}module {{.Path}} {{.Version}} -> {{.Update.Version}}{{end}}'

if "$GO" list -m -u -f "$mod_tmpl" all >"$tmp/mods" 2>"$tmp/mods.err"; then
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    echo "$line"
    outdated=1
  done <"$tmp/mods"
else
  skip_section modules "$(first_line "$tmp/mods.err")"
fi

# --- 2. Go toolchain patch -------------------------------------------------
declared="$(awk '$1 == "go" { print $2; exit }' go.mod)"
if [ -z "$declared" ]; then
  skip_section toolchain "no go directive in go.mod"
elif "$GO" list -m -versions go >"$tmp/gov" 2>"$tmp/gov.err"; then
  case "$declared" in
    *.*.*) minor="${declared%.*}" ;;
    *)     minor="$declared" ;;
  esac
  latest="$(tr ' ' '\n' <"$tmp/gov" |
    grep -E "^${minor}\.[0-9]+\$" |
    sort -V | tail -1 || true)"
  if [ -n "$latest" ] && version_lt "$declared" "$latest"; then
    echo "toolchain go.mod go $declared -> $latest"
    outdated=1
  fi
else
  skip_section toolchain "$(first_line "$tmp/gov.err")"
fi

# --- 3. GitHub Action pins -------------------------------------------------
: >"$tmp/tagcache"
: >"$tmp/seen"
action_failed=""

# Prints the newest tag for owner/repo whose shape matches the pin's prefix
# ("v6.4.0" -> "v"), cached per run. A repo's "latest release" is not always a
# version tag: github/codeql-action ships codeql-bundle-* releases alongside
# its vX.Y.Z tags, hence the tag-list fallback.
latest_tag() {
  local slug="$1" prefix="$2" key hit tag
  key="${slug}|${prefix}"
  hit="$(grep -m1 -F "${key} " "$tmp/tagcache" || true)"
  if [ -n "$hit" ]; then
    printf '%s\n' "${hit#* }"
    return 0
  fi
  tag="$("$GH" api "repos/${slug}/releases/latest" --jq .tag_name 2>"$tmp/gh.err" || true)"
  case "$tag" in
    "${prefix}"[0-9]*) ;;
    *) tag="" ;;
  esac
  if [ -z "$tag" ]; then
    tag="$("$GH" api "repos/${slug}/tags" --jq '.[].name' 2>>"$tmp/gh.err" |
      grep -E "^${prefix}[0-9]+(\.[0-9]+)*\$" | sort -V | tail -1 || true)"
  fi
  if [ -z "$tag" ]; then
    return 1
  fi
  printf '%s %s\n' "$key" "$tag" >>"$tmp/tagcache"
  printf '%s\n' "$tag"
}

extract_uses='
/^[[:space:]]*(-[[:space:]]*)?uses:[[:space:]]*/ {
  line = $0
  sub(/^[^:]*uses:[[:space:]]*/, "", line)
  comment = ""
  if (match(line, /#/)) {
    comment = substr(line, RSTART + 1)
    line = substr(line, 1, RSTART - 1)
  }
  gsub(/^[[:space:]]+|[[:space:]]+$/, "", line)
  gsub(/^[[:space:]]+|[[:space:]]+$/, "", comment)
  gsub(/^["'\''"]|["'\''"]$/, "", line)
  if (line != "") print FILENAME "\t" line "\t" comment
}'

shopt -s nullglob
workflows=(.github/workflows/*.yml .github/workflows/*.yaml)
shopt -u nullglob

if [ ${#workflows[@]} -gt 0 ]; then
  awk "$extract_uses" "${workflows[@]}" >"$tmp/uses"
  while IFS=$'\t' read -r file uses comment; do
    case "$uses" in
      ./*|docker://*|'') continue ;;
      *@*) ;;
      *) continue ;;
    esac

    slug="$(printf '%s\n' "${uses%@*}" | cut -d/ -f1,2)"
    ref="${uses##*@}"

    # A SHA pin is only readable through its trailing "# vX.Y.Z" comment.
    if printf '%s' "$ref" | grep -qiE '^[0-9a-f]{40}$'; then
      current="$(printf '%s\n' "$comment" | grep -oE '^v?[0-9]+(\.[0-9]+)*' || true)"
    else
      current="$ref"
    fi

    # A bare SHA or a moving ref (@main) carries no version to compare against.
    # That is a property of the pin, not of the network, so it is a finding
    # rather than a skipped section.
    if ! printf '%s' "$current" | grep -qE '^v?[0-9]+(\.[0-9]+)*$'; then
      if ! grep -qx "unpinned ${slug}" "$tmp/seen"; then
        echo "action ${slug} unpinned-by-tag ${ref} (${file})"
        echo "unpinned ${slug}" >>"$tmp/seen"
        outdated=1
      fi
      continue
    fi

    if grep -qx "${slug}@${current}" "$tmp/seen"; then
      continue
    fi
    echo "${slug}@${current}" >>"$tmp/seen"

    prefix="$(printf '%s' "$current" | sed -e 's/[0-9].*//')"
    if ! latest="$(latest_tag "$slug" "$prefix")"; then
      action_failed="$slug"
      continue
    fi
    if version_lt "$current" "$latest"; then
      echo "action ${slug} ${current} -> ${latest} (${file})"
      outdated=1
    fi
  done <"$tmp/uses"
fi

if [ -n "$action_failed" ]; then
  skip_section actions "$(first_line "$tmp/gh.err")"
fi

# --- verdict ---------------------------------------------------------------
if [ "$unverified" = "1" ]; then
  echo
  echo "dependency check incomplete — rerun with network, or set DEPS_CHECK_OFFLINE_OK=1 to accept the gap"
  exit 2
fi
if [ "$outdated" = "1" ]; then
  echo
  echo "outdated or unverifiable dependencies found. Fix: go get -u ./... && go mod tidy; bump the go directive in go.mod; update the action pin and its trailing # vX.Y.Z comment"
  exit 1
fi
echo "dependencies current"
