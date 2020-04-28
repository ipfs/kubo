#!/usr/bin/env bash
#
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test unix API transport"

. lib/test-lib.sh

test_init_ipfs

# We can't use the trash dir as the full name must be longer less than 108 bytes
# long (because that's the max unix domain socket path length).
SOCKDIR="$(mktemp -d "${TMPDIR:-/tmp}/unix-api-sharness.XXXXXX")"

test_expect_success "configure" '
  peerid=$(ipfs config Identity.PeerID) &&
  ipfs config Addresses.API "/unix/$SOCKDIR/sock"
'

test_launch_ipfs_daemon

test_expect_success "client works" '
  printf "$peerid" >expected &&
  ipfs --api="/unix/$SOCKDIR/sock" id -f="<id>" >actual &&
  test_cmp expected actual
'

test_kill_ipfs_daemon
test_done
