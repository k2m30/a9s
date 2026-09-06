#!/usr/bin/env bash
# Moves the per-task fragments in changelog.d/ into CHANGELOG.md's Unreleased
# section and deletes them. See changelog.d/README.md for the fragment format.
# Assembly is the only writer of CHANGELOG.md outside a release, so a task that
# writes only its own fragment can never conflict with another task there.
#
# Modes:
#   (default)  assemble, then delete the consumed fragments
#   --check    exit 1 while changelog.d/ still holds a fragment
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fragments_dir="$repo_root/changelog.d"
changelog="$repo_root/CHANGELOG.md"

# Keep a Changelog section order, used for a section Unreleased does not have yet.
readonly SECTION_ORDER="Added Changed Deprecated Removed Fixed Security"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

find "$fragments_dir" -maxdepth 1 -name '*.md' ! -name 'README.md' 2>/dev/null | LC_ALL=C sort > "$tmp/fragments"

if [ "${1:-assemble}" = "--check" ]; then
  if [ ! -s "$tmp/fragments" ]; then
    echo "PASS: no unassembled changelog.d fragments"
    exit 0
  fi
  echo "FAIL: changelog.d still holds unassembled fragments — run 'make changelog':"
  sed "s|^$repo_root/|  |" "$tmp/fragments"
  exit 1
fi

if [ "${1:-assemble}" != "assemble" ]; then
  echo "usage: $(basename "$0") [assemble|--check]" >&2
  exit 2
fi

if [ ! -s "$tmp/fragments" ]; then
  echo "nothing to assemble: changelog.d holds no fragment"
  exit 0
fi

# Split every fragment into one file per section, in file-name order.
while IFS= read -r fragment; do
  awk -v tmpdir="$tmp" '
    /^## / {
      section = substr($0, 4)
      sub(/[ \t]+$/, "", section)
      next
    }
    section != "" && NF { print >> (tmpdir "/sec." section) }
  ' "$fragment"
done < "$tmp/fragments"

awk -v tmpdir="$tmp" -v order="$SECTION_ORDER" '
  function trimmed_end(sec,   last) {
    last = count[sec]
    while (last > 0 && body[sec, last - 1] ~ /^[ \t]*$/) { last-- }
    return last
  }
  function extra(sec,   f, line, printed) {
    f = tmpdir "/sec." sec
    printed = 0
    while ((getline line < f) > 0) { print line; printed = 1 }
    close(f)
    return printed
  }
  function emit(   i, line, sec, last, j, k, nsplit, names) {
    sec = ""
    for (i = 0; i < n; i++) {
      line = buf[i]
      if (line ~ /^### /) {
        sec = substr(line, 5)
        sub(/[ \t]+$/, "", sec)
        # A section named twice inside Unreleased is one section: its bodies
        # merge under the first heading, which is what the fragments assume.
        if (!seen[sec]) { seen[sec] = 1; seq[nseen++] = sec }
        continue
      }
      body[sec, count[sec]++] = line
    }
    last = trimmed_end("")
    for (j = 0; j < last; j++) { print body["", j] }
    for (i = 0; i < nseen; i++) {
      sec = seq[i]
      print ""
      print "### " sec
      print ""
      last = trimmed_end(sec)
      for (j = 0; j < last; j++) {
        if (j == 0 && body[sec, j] ~ /^[ \t]*$/) { continue }
        print body[sec, j]
      }
      extra(sec)
    }
    nsplit = split(order, names, " ")
    for (k = 1; k <= nsplit; k++) {
      if (seen[names[k]]) { continue }
      f = tmpdir "/sec." names[k]
      if ((getline line < f) <= 0) { close(f); continue }
      close(f)
      print ""
      print "### " names[k]
      print ""
      extra(names[k])
    }
    print ""
  }
  !inside && /^## \[Unreleased\]/ { print; inside = 1; next }
  inside && /^## \[/ { emit(); inside = 0; print; next }
  inside { buf[n++] = $0; next }
  { print }
  END { if (inside) emit() }
' "$changelog" > "$tmp/CHANGELOG.md"

mv "$tmp/CHANGELOG.md" "$changelog"

while IFS= read -r fragment; do
  rm -f "$fragment"
  echo "assembled ${fragment#"$repo_root"/}"
done < "$tmp/fragments"
