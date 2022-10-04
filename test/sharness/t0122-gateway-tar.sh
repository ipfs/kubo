#!/usr/bin/env bash

test_description="Test HTTP Gateway TAR (application/x-tar) Support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon_without_network

DIR_HASH="bafybeiczqj6w5tggtshvlyevr24drgrboffuepe2lxeojsnwmfw4tpttzu"

test_expect_success "Add directory to test with" '
  ipfs dag import ../t0122-gateway-tar-data/foo.car
'
test_expect_success "GET TAR with format=tar and extract" '
  curl "http://127.0.0.1:$GWAY_PORT/ipfs/$DIR_HASH?format=tar" | tar -x
'

test_expect_success "GET TAR with 'Accept: application/x-tar' and extract" '
  curl -H "Accept: application/x-tar" "http://127.0.0.1:$GWAY_PORT/ipfs/$DIR_HASH" | tar -x
'

test_expect_success "GET TAR with format=tar has expected Content-Type" '
  curl -svX GET "http://127.0.0.1:$GWAY_PORT/ipfs/$DIR_HASH?format=tar" > curl_output_filename 2>&1 &&
  cat curl_output_filename &&
  grep "< Content-Type: application/x-tar" curl_output_filename
'

test_expect_success "GET TAR with 'Accept: application/x-tar' has expected Content-Type" '
  curl -svX GET -H "Accept: application/x-tar" "http://127.0.0.1:$GWAY_PORT/ipfs/$DIR_HASH" > curl_output_filename 2>&1 &&
  cat curl_output_filename &&
  grep "< Content-Type: application/x-tar" curl_output_filename
'

test_kill_ipfs_daemon

test_done
