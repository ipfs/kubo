#!/usr/bin/env bash

test_description="Test HTTP Gateway TAR (application/x-tar) Support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon_without_network

FOO_HASH="bafybeiczqj6w5tggtshvlyevr24drgrboffuepe2lxeojsnwmfw4tpttzu"

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

# outside-root.car --> bafybeicaj7kvxpcv4neaqzwhrqqmdstu4dhrwfpknrgebq6nzcecfucvyu
# inside-root.car --> bafybeibfevfxlvxp5vxobr5oapczpf7resxnleb7tkqmdorc4gl5cdva3y

test_kill_ipfs_daemon

test_done
