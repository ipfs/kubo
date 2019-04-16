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

TMPDIR="$(mktemp -d)"
cp -r . "$TMPDIR"
( cd "$TMPDIR" &&
  echo $PWD &&
  go mod vendor &&
  (git describe --always --match=NeVeRmAtCh --dirty 2>/dev/null || true) > .tarball &&
  chmod -R u=rwX,go=rX "$TMPDIR" # normalize permissions
  tar -czf "$OUTPUT" --exclude="./.git" .
  )

rm -rf "$TMPDIR"
