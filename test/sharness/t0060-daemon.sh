#!/bin/sh
#
# Copyright (c) 2014 Juan Batiz-Benet
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test daemon command"

. lib/test-lib.sh

# this needs to be in a different test than "ipfs daemon --init" below
test_expect_success "setup IPFS_PATH" '
  IPFS_PATH="$(pwd)/.ipfs"
'

# NOTE: this should remove bootstrap peers (needs a flag)
test_expect_success "ipfs daemon --init launches" '
  ipfs daemon --init >actual_daemon 2>daemon_err &
'

# this is like "'ipfs daemon' is ready" in test_launch_ipfs_daemon(), see test-lib.sh
test_expect_success "initialization ended" '
  IPFS_PID=$! &&
  pollEndpoint -ep=/version -v -tout=1s -tries=60 2>poll_apierr > poll_apiout ||
  test_fsh cat actual_daemon || test_fsh cat daemon_err || test_fsh cat poll_apierr || test_fsh cat poll_apiout
'

# this is lifted straight from t0020-init.sh
test_expect_success "ipfs peer id looks good" '
  PEERID=$(ipfs config Identity.PeerID) &&
  echo $PEERID | tr -dC "[:alnum:]" | wc -c | tr -d " " >actual &&
  echo "46" >expected &&
  test_cmp_repeat_10_sec expected actual
'

# This is like t0020-init.sh "ipfs init output looks good"
#
# Unfortunately the line:
#
#   API server listening on /ip4/127.0.0.1/tcp/5001
#
# sometimes doesn't show up, so we cannot use test_expect_success yet.
#
test_expect_failure "ipfs daemon output looks good" '
  STARTFILE="ipfs cat /ipfs/$HASH_WELCOME_DOCS/readme" &&
  echo "Initializing daemon..." >expected &&
  echo "initializing ipfs node at $IPFS_PATH" >>expected &&
  echo "generating 4096-bit RSA keypair...done" >>expected &&
  echo "peer identity: $PEERID" >>expected &&
  echo "to get started, enter:" >>expected &&
  printf "\\n\\t$STARTFILE\\n\\n" >>expected &&
  echo "API server listening on /ip4/127.0.0.1/tcp/5001" >>expected &&
  echo "Gateway (readonly) server listening on /ip4/127.0.0.1/tcp/8080" >>expected &&
  test_cmp_repeat_10_sec expected actual_daemon
'

test_expect_success ".ipfs/ has been created" '
  test -d ".ipfs" &&
  test -f ".ipfs/config" &&
  test -d ".ipfs/datastore" ||
  test_fsh ls .ipfs
'

# begin same as in t0010

test_expect_success "ipfs version succeeds" '
	ipfs version >version.txt
'

test_expect_success "ipfs version output looks good" '
	cat version.txt | egrep "^ipfs version [0-9]+\.[0-9]+\.[0-9]" >/dev/null ||
	test_fsh cat version.txt
'

test_expect_success "ipfs help succeeds" '
	ipfs help >help.txt
'

test_expect_success "ipfs help output looks good" '
	cat help.txt | egrep -i "^Usage:" >/dev/null &&
	cat help.txt | egrep "ipfs .* <command>" >/dev/null ||
	test_fsh cat help.txt
'

# end same as in t0010

test_expect_success "daemon is still running" '
  kill -0 $IPFS_PID
'

test_expect_success "'ipfs daemon' can be killed" '
  test_kill_repeat_10_sec $IPFS_PID
'

test_expect_failure "'ipfs daemon' should be able to run with a pipe attached to stdin (issue #861)" '
  yes | ipfs daemon --init >daemon_out 2>daemon_err &
  pollEndpoint -ep=/version -v -tout=1s -tries=10 >poll_apiout 2>poll_apierr &&
  test_kill_repeat_10_sec $! ||
  test_fsh cat daemon_out || test_fsh cat daemon_err || test_fsh cat poll_apiout || test_fsh cat poll_apierr
'

test_done
