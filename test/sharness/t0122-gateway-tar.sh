#!/usr/bin/env bash

test_description="Test HTTP Gateway TAR (application/x-tar) Support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon_without_network

FOO_CID="bafybeiczqj6w5tggtshvlyevr24drgrboffuepe2lxeojsnwmfw4tpttzu"
OUTSIDE_ROOT_CID="bafybeicaj7kvxpcv4neaqzwhrqqmdstu4dhrwfpknrgebq6nzcecfucvyu"
INSIDE_ROOT_CID="bafybeibfevfxlvxp5vxobr5oapczpf7resxnleb7tkqmdorc4gl5cdva3y"

test_expect_success "Add files and directories to test with" '
  ipfs dag import ../t0122-gateway-tar-data/foo.car > import_output &&
  test_should_contain $FOO_CID import_output &&
  ipfs dag import ../t0122-gateway-tar-data/outside-root.car > import_output &&
  test_should_contain $OUTSIDE_ROOT_CID import_output &&
  ipfs dag import ../t0122-gateway-tar-data/inside-root.car > import_output &&
  test_should_contain $INSIDE_ROOT_CID import_output
'

test_expect_success "GET TAR with format=tar and extract" '
  curl "http://127.0.0.1:$GWAY_PORT/ipfs/$FOO_CID?format=tar" | tar -x
'

test_expect_success "GET TAR with 'Accept: application/x-tar' and extract" '
  curl -H "Accept: application/x-tar" "http://127.0.0.1:$GWAY_PORT/ipfs/$FOO_CID" | tar -x
'

test_expect_success "GET TAR with format=tar has expected Content-Type" '
  curl -sD - "http://127.0.0.1:$GWAY_PORT/ipfs/$FOO_CID?format=tar" > curl_output_filename 2>&1 &&
  test_should_contain "Content-Type: application/x-tar" curl_output_filename
'

test_expect_success "GET TAR with 'Accept: application/x-tar' has expected Content-Type" '
  curl -sD - -H "Accept: application/x-tar" "http://127.0.0.1:$GWAY_PORT/ipfs/$FOO_CID" > curl_output_filename 2>&1 &&
  test_should_contain "Content-Type: application/x-tar" curl_output_filename
'

test_expect_success "GET TAR with relative paths outside root fails" '
  ! curl "http://127.0.0.1:$GWAY_PORT/ipfs/$OUTSIDE_ROOT_CID?format=tar"
'

test_expect_success "GET TAR with relative paths inside root works" '
  curl "http://127.0.0.1:$GWAY_PORT/ipfs/$INSIDE_ROOT_CID?format=tar" | tar -x
'

test_kill_ipfs_daemon

test_done
