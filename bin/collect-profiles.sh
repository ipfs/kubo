#!/usr/bin/env bash

# collect-profiles.sh
#
# Collects go profile information from a running `ipfs` daemon.
# Creates an archive including the profiles, profile graph svgs,
# ...and where available, a copy of the `ipfs` binary on the PATH.
#
# Please run this script and attach the profile archive it creates
# when reporting bugs at https://github.com/ipfs/go-ipfs/issues

set -euo pipefail
IFS=$'\n\t'

SOURCE_URL="${1:-http://127.0.0.1:5001}"
tmpdir=$(mktemp -d)
export PPROF_TMPDIR="$tmpdir"
pushd "$tmpdir" > /dev/null

if command -v ipfs > /dev/null 2>&1; then
  cp "$(command -v ipfs)" ipfs
fi

echo Collecting goroutine stacks
curl -s -o goroutines.stacks "$SOURCE_URL"'/debug/pprof/goroutine?debug=2'

echo Collecting goroutine profile
go tool pprof -symbolize=remote -svg -output goroutine.svg "$SOURCE_URL/debug/pprof/goroutine"

echo Collecting heap profile
go tool pprof -symbolize=remote -svg -output heap.svg "$SOURCE_URL/debug/pprof/heap"

echo "Collecting cpu profile (~30s)"
go tool pprof -symbolize=remote -svg -output cpu.svg "$SOURCE_URL/debug/pprof/profile"

echo "Enabling mutex profiling"
curl -X POST "$SOURCE_URL"'/debug/pprof-mutex/?fraction=4'

echo "Waiting for mutex data to be updated (30s)"
sleep 30
curl -s -o mutex.txt "$SOURCE_URL"'/debug/pprof/mutex?debug=2'
go tool pprof -symbolize=remote -svg -output mutex.svg "$SOURCE_URL/debug/pprof/mutex"

echo "Disabling mutex profiling"
curl -X POST "$SOURCE_URL"'/debug/pprof-mutex/?fraction=0'

OUTPUT_NAME=ipfs-profile-$(uname -n)-$(date +'%Y-%m-%dT%H:%M:%S%z').tar.gz
echo "Creating $OUTPUT_NAME"
popd > /dev/null
tar czf "./$OUTPUT_NAME" -C "$tmpdir" .
rm -rf "$tmpdir"
