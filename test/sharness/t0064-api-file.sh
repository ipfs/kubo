#!/usr/bin/env bash
#
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test api file"

. lib/test-lib.sh

test_init_ipfs


test_launch_ipfs_daemon
test_kill_ipfs_daemon

test_expect_success "version always works" '
  ipfs version >/dev/null
'

test_expect_success "swarm peers fails when offline" '
  test_must_fail ipfs swarm peers >/dev/null
'

test_expect_success "swarm peers fails when offline and API specified" '
  test_must_fail ipfs swarm peers --api="$API_MADDR" >/dev/null
'

test_expect_success "pin ls succeeds when offline" '
  ipfs pin ls >/dev/null
'

test_expect_success "pin ls fails when offline and API specified" '
  test_must_fail ipfs pin ls --api="$API_MADDR" >/dev/null
'

test_expect_success "id succeeds when offline" '
  ipfs id >/dev/null
'

test_expect_success "id fails when offline API specified" '
  test_must_fail ipfs id --api="$API_MADDR" >/dev/null
'

test_expect_success "create API file" '
  echo "$API_MADDR" > "$IPFS_PATH/api"
'

test_expect_success "version always works" '
  ipfs version >/dev/null
'

test_expect_success "id succeeds when offline and API file exists" '
  ipfs id >/dev/null
'

test_expect_success "pin ls succeeds when offline and API file exists" '
  ipfs pin ls >/dev/null
'

test_launch_ipfs_daemon

test_expect_success "version always works" '
  ipfs version >/dev/null
'

test_expect_success "id succeeds when online" '
  ipfs id >/dev/null
'

test_expect_success "swarm peers succeeds when online" '
  ipfs swarm peers >/dev/null
'

test_expect_success "pin ls succeeds when online" '
  ipfs pin ls >/dev/null
'

test_expect_success "remove API file when daemon is running" '
  rm "$IPFS_PATH/api"
'

test_expect_success "version always works" '
  ipfs version >/dev/null
'

test_expect_success "swarm peers fails when the API file is missing" '
  test_must_fail ipfs swarm peers >/dev/null
'

test_expect_success "id fails when daemon is running but API file is missing (locks repo)" '
  test_must_fail ipfs pin ls >/dev/null
'

test_expect_success "pin ls fails when daemon is running but API file is missing (locks repo)" '
  test_must_fail ipfs pin ls >/dev/null
'

test_kill_ipfs_daemon

APIPORT=32563

test_expect_success "Verify gateway file diallable while on unspecified" '
  ipfs config Addresses.API /ip4/0.0.0.0/tcp/$APIPORT &&
  test_launch_ipfs_daemon &&
  cat "$IPFS_PATH/api" > api_file_actual &&
  echo -n "/ip4/127.0.0.1/tcp/$APIPORT" > api_file_expected &&
  test_cmp api_file_expected api_file_actual
'

test_kill_ipfs_daemon

test_done
