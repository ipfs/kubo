#!/usr/bin/env bash
# vim: set expandtab sw=2 ts=2:

# bash safe mode
set -euo pipefail
IFS=$'\n\t'


OUTPUT=$(realpath ${1:-go-ipfs-source.tar.gz})

TMPDIR="$(mktemp -d)"
NEWIPFS="$TMPDIR/github.com/ipfs/go-ipfs"
mkdir -p "$NEWIPFS"
cp -r . "$NEWIPFS"
( cd "$NEWIPFS" &&
  echo $PWD &&
  GOPATH="$TMPDIR" gx install --local &&
  (git rev-parse --short HEAD || true) > .tarball &&
  tar -czf "$OUTPUT" --exclude="./.git" .
)

rm -rf "$TMPDIR"
