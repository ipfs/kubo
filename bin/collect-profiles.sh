#!/usr/bin/env bash
set -x
set -euo pipefail
IFS=$'\n\t'

HTTP_API="${1:-localhost:5001}"
tmpdir=$(mktemp -d)
export PPROF_TMPDIR="$tmpdir"
pushd "$tmpdir"

echo Collecting goroutine stacks
curl -o goroutines.stacks "http://$HTTP_API"'/debug/pprof/goroutine?debug=2'

echo Collecting goroutine profile
go tool pprof -symbolize=remote -svg -output goroutine.svg "http://$HTTP_API/debug/pprof/goroutine"

echo Collecting heap profile
go tool pprof -symbolize=remote -svg -output heap.svg "http://$HTTP_API/debug/pprof/heap"

echo "Collecting cpu profile (~30s)"
go tool pprof -symbolize=remote -svg -output cpu.svg "http://$HTTP_API/debug/pprof/profile"

echo "Enabling mutex profiling"
curl -X POST -v "http://$HTTP_API"'/debug/pprof-mutex/?fraction=4'

echo "Waiting for mutex data to be updated (30s)"
sleep 30
curl -o mutex.txt "http://$HTTP_API"'/debug/pprof/mutex?debug=2'
go tool pprof -symbolize=remote -svg -output mutex.svg "http://$HTTP_API/debug/pprof/mutex"
echo "Disabling mutex profiling"
curl -X POST -v "http://$HTTP_API"'/debug/pprof-mutex/?fraction=0'

popd
tar cvzf "./ipfs-profile-$(uname -n)-$(date -Iseconds).tar.gz" -C "$tmpdir" .
rm -rf "$tmpdir"




