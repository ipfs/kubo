#!/usr/bin/env bash

test_description="Test HTTP Gateway TAR (application/x-tar) Support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon_without_network

FOO_HASH="bafybeiczqj6w5tggtshvlyevr24drgrboffuepe2lxeojsnwmfw4tpttzu"
OUTSIDE_ROOT_HASH="bafybeicaj7kvxpcv4neaqzwhrqqmdstu4dhrwfpknrgebq6nzcecfucvyu"
INSIDE_ROOT_HASH="bafybeibfevfxlvxp5vxobr5oapczpf7resxnleb7tkqmdorc4gl5cdva3y"

test_expect_success "Add directory to test with" '
  ipfs dag import ../t0122-gateway-tar-data/foo.car > import_output &&
  grep -q $FOO_HASH import_output
'

test_expect_success "GET TAR with format=tar and extract" '
  curl "http://127.0.0.1:$GWAY_PORT/ipfs/$FOO_HASH?format=tar" | tar -x
'

test_expect_success "GET TAR with 'Accept: application/x-tar' and extract" '
  curl -H "Accept: application/x-tar" "http://127.0.0.1:$GWAY_PORT/ipfs/$FOO_HASH" | tar -x
'

test_expect_success "GET TAR with format=tar has expected Content-Type" '
  curl -svX GET "http://127.0.0.1:$GWAY_PORT/ipfs/$FOO_HASH?format=tar" > curl_output_filename 2>&1 &&
  cat curl_output_filename &&
  grep "< Content-Type: application/x-tar" curl_output_filename
'

test_expect_success "GET TAR with 'Accept: application/x-tar' has expected Content-Type" '
  curl -svX GET -H "Accept: application/x-tar" "http://127.0.0.1:$GWAY_PORT/ipfs/$FOO_HASH" > curl_output_filename 2>&1 &&
  cat curl_output_filename &&
  grep "< Content-Type: application/x-tar" curl_output_filename
'

test_expect_success "Add directories with relative paths to test with" '
  ipfs dag import ../t0122-gateway-tar-data/outside-root.car > import_output &&
  grep -q $OUTSIDE_ROOT_HASH import_output &&
  ipfs dag import ../t0122-gateway-tar-data/inside-root.car > import_output &&
  grep -q $INSIDE_ROOT_HASH import_output
'

test_expect_success "GET TAR with relative paths outside root fails" '
  curl -i "http://127.0.0.1:$GWAY_PORT/ipfs/$OUTSIDE_ROOT_HASH?format=tar" > curl_output_filename &&
  grep -q "Trailer: X-Stream-Error" curl_output_filename &&
  grep -q "X-Stream-Error:" curl_output_filename
'

test_expect_success "GET TAR with relative paths inside root works" '
  curl -i "http://127.0.0.1:$GWAY_PORT/ipfs/$INSIDE_ROOT_HASH?format=tar" > curl_output_filename &&
  ! grep -q "X-Stream-Error:" curl_output_filename
'

test_kill_ipfs_daemon

test_done
