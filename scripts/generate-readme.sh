#!/bin/bash
# Generates README.md from docs/README.tmpl.md by replacing <!-- INCLUDE: file.md --> markers
# with contents from docs/shared/. Run via: make readme

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
TEMPLATE="$ROOT_DIR/docs/README.tmpl.md"
SHARED_DIR="$ROOT_DIR/docs/shared"

if [ ! -f "$TEMPLATE" ]; then
  echo "ERROR: Template not found: $TEMPLATE" >&2
  exit 1
fi

while IFS= read -r line; do
  if [[ "$line" =~ ^'<!-- INCLUDE: '(.+)' -->'$ ]]; then
    file="$SHARED_DIR/${BASH_REMATCH[1]}"
    if [ ! -f "$file" ]; then
      echo "ERROR: Snippet not found: $file" >&2
      exit 1
    fi
    cat "$file"
  else
    printf '%s\n' "$line"
  fi
done < "$TEMPLATE"
