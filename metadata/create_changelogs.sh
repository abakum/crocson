#!/usr/bin/env bash
set -euo pipefail

BUILD="$1"
VERSION="$2"

for LOCALE_DIR in metadata/*/changelogs; do
  ORIGINAL="${LOCALE_DIR}/${VERSION}.txt"
  if [ ! -f "$ORIGINAL" ]; then
    continue
  fi
  for i in 0 1 2 3; do
    VC=$((BUILD + i))
    LINK="${LOCALE_DIR}/${VC}.txt"
    ln -sf "${VERSION}.txt" "$LINK"
    echo "Linked: ${LINK} -> ${VERSION}.txt"
  done
done
