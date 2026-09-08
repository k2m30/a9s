#!/usr/bin/env bash
# Prints the files touched by more than one of the given task branches — the
# clash rule the team loop enforces before dispatching two tasks at once.
# Each branch's file set is `git diff --name-only <base>...<branch>`.
#
# Usage: scripts/task-file-overlap.sh task/a task/b [task/c ...]
# Exit 1 when the intersection is non-empty.
set -euo pipefail

if [ "$#" -lt 2 ]; then
  echo "usage: $(basename "$0") <branch> <branch> [<branch>...]" >&2
  exit 2
fi

cd "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

base=origin/main
if ! git rev-parse --verify --quiet "$base" > /dev/null; then
  base=main
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Named by position, never by the branch name: `tr / _` maps "task/a_b" and
# "task_a/b" to one file, so one branch's list silently replaced the other's
# and the clash rule reported PASS on a real overlap. "$@" in the same order
# is the index -> branch map both loops read.
i=0
for branch in "$@"; do
  git diff --name-only "$base...$branch" | LC_ALL=C sort -u > "$tmp/$i"
  i=$((i + 1))
done

overlap="$(cat "$tmp"/* | LC_ALL=C sort | uniq -d)"

if [ -z "$overlap" ]; then
  echo "PASS: no file touched by more than one of: $*"
  exit 0
fi

echo "FAIL: files touched by more than one task branch:"
while IFS= read -r file; do
  owners=""
  i=0
  for branch in "$@"; do
    if grep -qxF -- "$file" "$tmp/$i"; then
      owners="$owners $branch"
    fi
    i=$((i + 1))
  done
  echo "  $file —$owners"
done <<< "$overlap"
exit 1
