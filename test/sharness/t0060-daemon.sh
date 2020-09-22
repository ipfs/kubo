#!/usr/bin/env bash
#
# Copyright (c) 2014 Juan Batiz-Benet
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test daemon command"

. lib/test-lib.sh

test_expect_success "create badger config" '
  ipfs init --profile=badgerds,test > /dev/null &&
  cp "$IPFS_PATH/config" init-config
'

test_expect_success "cleanup repo" '
  rm -rf "$IPFS_PATH"
'

test_launch_ipfs_daemon --init --init-config="$(pwd)/init-config" --init-profile=test
test_kill_ipfs_daemon

test_expect_success "daemon initialization with existing config works" '
  ipfs config "Datastore.Spec.child.path" >actual &&
  test $(cat actual) = "badgerds" &&
  ipfs config Addresses > orig_addrs
'

test_expect_success "cleanup repo" '
  rm -rf "$IPFS_PATH"
'

test_launch_ipfs_daemon --init --init-config="$(pwd)/init-config" --init-profile=test,randomports
test_kill_ipfs_daemon

test_expect_success "daemon initialization with existing config + profiles works" '
  ipfs config Addresses >new_addrs &&
  test_expect_code 1 diff -q new_addrs orig_addrs
'

test_expect_success "cleanup repo" '
  rm -rf "$IPFS_PATH"
'

test_init_ipfs
test_launch_ipfs_daemon

# this errors if we didn't --init $IPFS_PATH correctly
test_expect_success "'ipfs config Identity.PeerID' works" '
  PEERID=$(ipfs config Identity.PeerID)
'

test_expect_success "'ipfs swarm addrs local' works" '
  ipfs swarm addrs local >local_addrs
'

test_expect_success "ipfs swarm addrs listen; works" '
  ipfs swarm addrs listen >listen_addrs
'

test_expect_success "ipfs peer id looks good" '
  test_check_peerid "$PEERID"
'

# this is for checking SetAllowedOrigins race condition for the api and gateway
# See https://github.com/ipfs/go-ipfs/pull/1966
test_expect_success "ipfs API works with the correct allowed origin port" '
  curl -s -X POST -H "Origin:http://localhost:$API_PORT" -I "http://$API_ADDR/api/v0/version"
'

test_expect_success "ipfs gateway works with the correct allowed origin port" '
  curl -s -X POST -H "Origin:http://localhost:$GWAY_PORT" -I "http://$GWAY_ADDR/api/v0/version"
'

test_expect_success "ipfs daemon output looks good" '
  STARTFILE="ipfs cat /ipfs/$HASH_WELCOME_DOCS/readme" &&
  echo "Initializing daemon..." >expected_daemon &&
  ipfs version --all >> expected_daemon &&
  sed "s/^/Swarm listening on /" listen_addrs >>expected_daemon &&
  sed "s/^/Swarm announcing /" local_addrs >>expected_daemon &&
  echo "API server listening on '$API_MADDR'" >>expected_daemon &&
  echo "WebUI: http://'$API_ADDR'/webui" >>expected_daemon &&
  echo "Gateway (readonly) server listening on '$GWAY_MADDR'" >>expected_daemon &&
  echo "Daemon is ready" >>expected_daemon &&
  test_cmp expected_daemon actual_daemon
'

test_expect_success ".ipfs/ has been created" '
  test -d ".ipfs" &&
  test -f ".ipfs/config" &&
  test -d ".ipfs/datastore" &&
  test -d ".ipfs/blocks" ||
  test_fsh ls -al .ipfs
'

# begin same as in t0010

test_expect_success "ipfs version succeeds" '
  ipfs version >version.txt
'

test_expect_success "ipfs version output looks good" '
  egrep "^ipfs version [0-9]+\.[0-9]+\.[0-9]" version.txt >/dev/null ||
  test_fsh cat version.txt
'

test_expect_success "ipfs version deps succeeds" '
  ipfs version deps >deps.txt
'

test_expect_success "ipfs version deps output looks good ( set \$GOIPFSTEST_SKIP_LOCAL_DEVTREE_DEPS_CHECK to skip this test )" '
  head -1 deps.txt | grep "go-ipfs@(devel)" &&
  [[ "$GOIPFSTEST_SKIP_LOCAL_DEVTREE_DEPS_CHECK" == "1" ]] ||
  [[ $(tail -n +2 deps.txt | egrep -v -c "^[^ @]+@v[^ @]+( => [^ @]+@v[^ @]+)?$") -eq 0 ]] ||
  test_fsh cat deps.txt
'

test_expect_success "ipfs help succeeds" '
  ipfs help >help.txt
'

test_expect_success "ipfs help output looks good" '
  egrep -i "^Usage" help.txt >/dev/null &&
  egrep "ipfs .* <command>" help.txt >/dev/null ||
  test_fsh cat help.txt
'

# check transport is encrypted
test_expect_success SOCAT "transport should be encrypted ( needs socat )" '
  socat - tcp:localhost:$SWARM_PORT,connect-timeout=1 > swarmnc < ../t0060-data/mss-ls &&
  grep -q "/tls" swarmnc &&
  test_must_fail grep -q "/plaintext/1.0.0" swarmnc ||
  test_fsh cat swarmnc
'

test_expect_success "output from streaming commands works" '
  test_expect_code 28 curl -X POST -m 5 http://localhost:$API_PORT/api/v0/stats/bw\?poll=true > statsout
'

test_expect_success "output looks good" '
  grep TotalIn statsout > /dev/null &&
  grep TotalOut statsout > /dev/null &&
  grep RateIn statsout > /dev/null &&
  grep RateOut statsout >/dev/null
'

# end same as in t0010

test_expect_success "daemon is still running" '
  kill -0 $IPFS_PID
'

test_expect_success "'ipfs daemon' can be killed" '
  test_kill_repeat_10_sec $IPFS_PID
'

test_expect_success "'ipfs daemon' should be able to run with a pipe attached to stdin (issue #861)" '
  yes | ipfs daemon >stdin_daemon_out 2>stdin_daemon_err &
  DAEMON_PID=$!
  test_wait_for_file 20 100ms "$IPFS_PATH/api" &&
  test_set_address_vars stdin_daemon_out
'

test_expect_success "daemon with pipe eventually becomes live" '
  pollEndpoint -host='$API_MADDR' -v -tout=1s -tries=10 >stdin_poll_apiout 2>stdin_poll_apierr &&
  test_kill_repeat_10_sec $DAEMON_PID ||
  test_fsh cat stdin_daemon_out || test_fsh cat stdin_daemon_err || test_fsh cat stdin_poll_apiout || test_fsh cat stdin_poll_apierr
'

test_expect_success "'ipfs daemon' cleans up when it fails to start" '
  test_must_fail ipfs daemon --routing=foobar &&
  test ! -e "$IPFS_PATH/repo.lock"
'

ulimit -S -n 512
TEST_ULIMIT_PRESET=1
test_launch_ipfs_daemon

test_expect_success "daemon raised its fd limit" '
  grep -v "setting file descriptor limit" actual_daemon > /dev/null
'

test_expect_success "daemon actually can handle 2048 file descriptors" '
  hang-fds -hold=2s 2000 '$API_MADDR' > /dev/null
'

test_expect_success "daemon didn't throw any errors" '
  test_expect_code 1 grep "too many open files" daemon_err
'

test_kill_ipfs_daemon

test_done
