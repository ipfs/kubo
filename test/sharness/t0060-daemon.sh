#!/bin/sh
#
# Copyright (c) 2014 Juan Batiz-Benet
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test daemon command"

. lib/test-lib.sh

gwyport=8080
apiport=5001

# TODO: randomize ports here once we add --config to ipfs daemon

# this needs to be in a different test than "ipfs daemon --init" below
test_expect_success "setup IPFS_PATH" '
  IPFS_PATH="$(pwd)/.ipfs" &&
  export IPFS_PATH
'

# NOTE: this should remove bootstrap peers (needs a flag)
# TODO(cryptix):
#  - we won't see daemon startup failure because we put the daemon in the background - fix: fork with exit code after api listen
#  - also default ports: might clash with local clients. Failure in that case isn't clear as well because pollEndpoint just uses the already running node
test_expect_success "ipfs daemon --init launches" '
  ipfs daemon --init >actual_daemon 2>daemon_err &
'

# this is like "'ipfs daemon' is ready" in test_launch_ipfs_daemon(), see test-lib.sh
test_expect_success "initialization ended" '
  IPFS_PID=$! &&
  pollEndpoint -ep=/version -v -tout=1s -tries=60 2>poll_apierr > poll_apiout ||
  test_fsh cat actual_daemon || test_fsh cat daemon_err || test_fsh cat poll_apierr || test_fsh cat poll_apiout
'

# this errors if daemon didnt --init $IPFS_PATH correctly
test_expect_success "'ipfs config Identity.PeerID' works" '
  PEERID=$(ipfs config Identity.PeerID)
'

test_expect_success "'ipfs swarm addrs local' works" '
  ipfs swarm addrs local >local_addrs
'

test_expect_success "ipfs peer id looks good" '
  test_check_peerid "$PEERID"
'

# this is for checking SetAllowedOrigins race condition for the api and gateway
# See https://github.com/ipfs/go-ipfs/pull/1966
test_expect_success "ipfs API works with the correct allowed origin port" '
  curl -s -X GET -H "Origin:http://localhost:$apiport" -I "http://localhost:$apiport/api/v0/version"
'

test_expect_success "ipfs gateway works with the correct allowed origin port" '
  curl -s -X GET -H "Origin:http://localhost:$gwyport" -I "http://localhost:$gwyport/api/v0/version"
'

# This is like t0020-init.sh "ipfs init output looks good"
#
# Unfortunately the line:
#
#   API server listening on /ip4/127.0.0.1/tcp/5001
#
# sometimes doesn't show up, so we cannot use test_expect_success yet.
#
test_expect_success "ipfs daemon output looks good" '
  STARTFILE="ipfs cat /ipfs/$HASH_WELCOME_DOCS/readme" &&
  echo "Initializing daemon..." >expected_daemon &&
  echo "initializing ipfs node at $IPFS_PATH" >>expected_daemon &&
  echo "generating 2048-bit RSA keypair...done" >>expected_daemon &&
  echo "peer identity: $PEERID" >>expected_daemon &&
  echo "to get started, enter:" >>expected_daemon &&
  printf "\\n\\t$STARTFILE\\n\\n" >>expected_daemon &&
  sed "s/^/Swarm listening on /" local_addrs >>expected_daemon &&
  echo "API server listening on /ip4/127.0.0.1/tcp/5001" >>expected_daemon &&
  echo "Gateway (readonly) server listening on /ip4/127.0.0.1/tcp/8080" >>expected_daemon &&
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

test_expect_success "ipfs help succeeds" '
	ipfs help >help.txt
'

test_expect_success "ipfs help output looks good" '
	egrep -i "^Usage:" help.txt >/dev/null &&
	egrep "ipfs .* <command>" help.txt >/dev/null ||
	test_fsh cat help.txt
'

# netcat (nc) is needed for the following test
test_expect_success "nc is available" '
	type nc >/dev/null
'

# check transport is encrypted
test_expect_success "transport should be encrypted" '
  nc -w 5 localhost 4001 >swarmnc &&
  grep -q "AES-256,AES-128" swarmnc &&
  test_must_fail grep -q "/multistream/1.0.0" swarmnc ||
	test_fsh cat swarmnc
'

test_expect_success "output from streaming commands works" '
	test_expect_code 28 curl -m 2 http://localhost:5001/api/v0/stats/bw\?poll=true > statsout
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
  yes | ipfs daemon --init >stdin_daemon_out 2>stdin_daemon_err &
  pollEndpoint -ep=/version -v -tout=1s -tries=10 >stdin_poll_apiout 2>stdin_poll_apierr &&
  test_kill_repeat_10_sec $! ||
  test_fsh cat stdin_daemon_out || test_fsh cat stdin_daemon_err || test_fsh cat stdin_poll_apiout || test_fsh cat stdin_poll_apierr
'

test_done
