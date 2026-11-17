#!/usr/bin/env bash
# Prints the packages whose acceptance tests a change needs (ADR-0007): the
# changed plugin folders, or every folder when anything outside them changed.
# Without a base revision it prints every folder.
set -euo pipefail
base=${1:-}
all=$(find internal/plugins -mindepth 1 -maxdepth 1 -type d | sort | sed 's#^#./#' | tr '\n' ' ')
if [ -z "$base" ]; then
  echo "$all"
  exit 0
fi
pkgs=()
while IFS= read -r f; do
  case "$f" in
    internal/plugins/*/*) pkgs+=("./$(cut -d/ -f1-3 <<<"$f")") ;;
    *.md | design/* | docs/* | examples/* | templates/* | LICENSE | .github/CODEOWNERS) ;;
    *) echo "$all"; exit 0 ;;
  esac
done < <(git diff --name-only "$base"...HEAD)
if [ ${#pkgs[@]} -gt 0 ]; then
  printf '%s\n' "${pkgs[@]}" | sort -u | tr '\n' ' '
fi
echo
