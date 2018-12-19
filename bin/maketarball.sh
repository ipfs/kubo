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
NEWIPFS="$TMPDIR/src/github.com/ipfs/go-ipfs"
mkdir -p "$NEWIPFS"
cp -r . "$NEWIPFS"
( cd "$NEWIPFS" &&
      echo $PWD &&
      GOPATH="$TMPDIR" gx install --local &&
      (git rev-parse --short HEAD || true) > .tarball &&
      chmod -R u=rwX,go=rX "$NEWIPFS" # normalize permissions
      tar -czf "$OUTPUT" --exclude="./.git" .
)

rm -rf "$TMPDIR"
