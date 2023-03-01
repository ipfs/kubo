#!/usr/bin/env bash

test_description="Miscellaneous Gateway Tests"

. lib/test-lib.sh

## ============================================================================
## HAMT-sharded file range test. We must execute this test first as we want to
## start from an empty repository and count the exact number of refs we want.
## ============================================================================

test_expect_success "ipfs init" '
  export IPFS_PATH="$(pwd)/.ipfs-for-hamt" &&
  ipfs init --empty-repo --profile=test > /dev/null
'

test_launch_ipfs_daemon_without_network

# HAMT_CID is a HAMT file.
HAMT_CID=bafybeibkzwf3ffl44yfej6ak44i7aly7rb4udhz5taskreec7qemmw5jiu

# HAMT_REFS_CID is root of a DAG that is a subset of HAMT_CID DAG
# representing minimal set of block necessary for the range request.
HAMT_REFS_CID=bafybeigcvrf7fvk7i3fdhxgk6saqdpw6spujwfxxkq5cshy5kxdjc674ua
HAMT_REFS_COUNT=48

test_expect_success "hamt: import fixture with necessary refs" '
  ipfs dag import ../t0110-gateway/hamt-refs.car &&
  ipfs refs local | wc -l | tr -d " " > refs_count_actual &&
  echo $HAMT_REFS_COUNT > refs_count_expected &&
  test_cmp refs_count_expected refs_count_actual
'

# we want to confirm that the code responsible for the range requests in regular
# files works with a minimal set of blocks, and does not fetch the entire thing
# (regression test)
test_expect_success "hamt: fetch directory from gateway in offline mode" '
  curl --max-time 30 -sD - -H "Range: bytes=2000-2002, 40000000000-40000000002" http://127.0.0.1:$GWAY_PORT/ipfs/$HAMT_CID/ > hamt_range_request &&
  ipfs refs local | wc -l | tr -d " " > refs_count_actual &&
  echo $HAMT_REFS_COUNT > refs_count_expected &&
  test_cmp refs_count_expected refs_count_actual &&
  test_should_contain "Content-Range: bytes 2000-2002/87186935127" hamt_range_request &&
  test_should_contain "Content-Type: application/octet-stream" hamt_range_request &&
  test_should_contain "Content-Range: bytes 40000000000-40000000002/87186935127" hamt_range_request
'

test_kill_ipfs_daemon

test_done
