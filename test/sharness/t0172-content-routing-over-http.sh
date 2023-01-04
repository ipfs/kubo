#!/usr/bin/env bash

test_description="Test content routing over HTTP"

. lib/test-lib.sh


if ! test_have_prereq SOCAT; then
  skip_all="skipping '$test_description': socat is not available"
  test_done
fi

test_init_ipfs

# Run listener on a free port to log HTTP requests sent by Kubo in Routing.Type=auto mode
export ROUTER_PORT=$(comm -23 <(seq 49152 65535 | sort) <(ss -Htan | awk '{print $4}' | cut -d':' -f2) | head -n 1)
export IPFS_HTTP_ROUTERS="http://127.0.0.1:$ROUTER_PORT"

test_launch_ipfs_daemon

test_expect_success "start HTTP router proxy" '
  socat TCP-LISTEN:$ROUTER_PORT,reuseaddr,fork,bind=127.0.0.1 STDOUT > http_requests &
  NCPID=$!
  test_wait_for_file 50 100ms http_requests
'

## HTTP GETs

test_expect_success 'create unique CID without adding it to the local datastore' '
  WANT_CID=$(date +"%FT%T.%N%z" | ipfs add -qn)
'

test_expect_success 'expect HTTP request for unknown CID' '
  ipfs block stat "$WANT_CID" &
  test_wait_output_n_lines_60_sec http_requests 3 &&
  test_should_contain "GET /routing/v1/providers/$WANT_CID" http_requests
'

## HTTP PUTs

test_expect_success 'add new CID to the local datastore' '
  ADD_CID=$(date +"%FT%T.%N%z" | ipfs add -q)
'

# cid.contact supports GET-only: https://github.com/ipfs/kubo/issues/9504
# which means no announcements over HTTP should be made.
test_expect_success 'expect no HTTP requests to be sent with locally added CID' '
  test_should_not_contain "$ADD_CID" http_requests
'

test_expect_success "stop nc" '
  kill "$NCPID" && wait "$NCPID" || true
'

test_kill_ipfs_daemon
test_done
