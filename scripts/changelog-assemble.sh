#!/usr/bin/env bash
# Assembles CHANGELOG.md's Unreleased section from the per-task fragments in
# changelog.d/. See changelog.d/README.md for the fragment format.
#
# Modes:
#   (default)  print the assembled Unreleased section to stdout
#   --check    exit 1 when a fragment line is missing from CHANGELOG.md
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fragments_dir="$repo_root/changelog.d"
changelog="$repo_root/CHANGELOG.md"

mode="${1:-print}"

fragment_files() {
  find "$fragments_dir" -maxdepth 1 -name '*.md' ! -name 'README.md' | LC_ALL=C sort
}

# unreleased_body prints CHANGELOG.md's Unreleased section, exclusive of the
# heading and of the next release heading.
unreleased_body() {
  awk '/^## \[Unreleased\]/ {inside=1; next} /^## \[/ {inside=0} inside {print}' "$changelog"
}

# assembled prints the merged fragment sections in file-name order: one
# "### <Section>" per section named by any fragment, carrying that section's
# lines from every fragment, in Keep a Changelog order.
assembled() {
  local section
  for section in Added Changed Fixed Removed; do
    local body
    body="$(
      while IFS= read -r f; do
        [ -n "$f" ] || continue
        awk -v want="## $section" '
          $0 == want {inside=1; next}
          /^## / {inside=0}
          inside {print}
        ' "$f"
      done < <(fragment_files) | sed '/^[[:space:]]*$/d'
    )"
    [ -n "$body" ] || continue
    printf '### %s\n\n%s\n\n' "$section" "$body"
  done
}

case "$mode" in
print)
  assembled
  ;;
--check)
  missing=0
  body="$(unreleased_body)"
  while IFS= read -r line; do
    case "$line" in
    '### '*) continue ;;
    '') continue ;;
    esac
    if ! printf '%s\n' "$body" | grep -qF -- "$line"; then
      echo "FAIL: changelog.d fragment line not in CHANGELOG.md Unreleased — run 'make changelog':"
      echo "  $line"
      missing=1
    fi
  done < <(assembled)
  if [ "$missing" -ne 0 ]; then
    exit 1
  fi
  echo "PASS: no unassembled changelog.d fragments"
  ;;
*)
  echo "usage: $(basename "$0") [print|--check]" >&2
  exit 2
  ;;
esac
