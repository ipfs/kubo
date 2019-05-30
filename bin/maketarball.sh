#!/usr/bin/env bash
# vim: set expandtab sw=2 ts=2:

# bash safe mode
set -euo pipefail
IFS=$'\n\t'

# readlink doesn't work on macos
OUTPUT="${1:-go-ipfs-source.tar.gz}"
if ! [[ "$OUTPUT" = /* ]]; then
  OUTPUT="$PWD/$OUTPUT"
fi

GOCC=${GOCC=go}

TEMP="$(mktemp -d)"
cp -r . "$TEMP"
( cd "$TEMP" &&
  echo $PWD &&
  $GOCC mod vendor &&
  (git describe --always --match=NeVeRmAtCh --dirty 2>/dev/null || true) > .tarball &&
  chmod -R u=rwX,go=rX "$TEMP" # normalize permissions
  tar -czf "$OUTPUT" --exclude="./.git" .
  )

rm -rf "$TEMP"
