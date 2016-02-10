#!/bin/sh
#
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test mount command in conjunction with publishing"

. lib/test-lib.sh

# if in travis CI, dont test mount (no fuse)
if ! test_have_prereq FUSE; then
	skip_all='skipping mount tests, fuse not available'

	test_done
fi

test_init_ipfs

# start iptb
iptb init -n 2 -f --bootstrap=star --port=$PORT_SWARM
iptb start --wait
BADDR="/ip4/127.0.0.1/tcp/$PORT_SWARM/ipfs/"
ADDR1="${BADDR}$(iptb get id 0)"
ADDR2="${BADDR}$(iptb get id 1)"

# bootstrap to the iptb peers
test_expect_success "bootstrap to iptb peers" '
  ipfs bootstrap add '$ADDR1' &&
  ipfs bootstrap add '$ADDR2'
'

# launch the daemon
test_launch_ipfs_daemon

# wait for peer bootstrapping
# TODO(noffle): this is very fragile -- how can we wait for this to happen for sure?
sleep 3

# pre-mount publish
HASH=$(echo 'hello warld' | ipfs add -q)
test_expect_success "can publish before mounting /ipns" '
  ipfs name publish '$HASH'
'

# mount
test_mount_ipfs

test_expect_success "cannot publish after mounting /ipns" '
  echo "Error: You cannot manually publish while IPNS is mounted." >expected &&
  test_must_fail ipfs name publish '$HASH' 2>actual &&
  test_cmp expected actual
'


test_expect_success "unmount /ipns out-of-band" '
  fusermount -u ipns
'

# wait a moment for the daemon to notice and clean up
# TODO(noffle): this is very fragile -- how can we wait for this to happen for sure?
sleep 2

test_expect_success "can publish after unmounting /ipns" '
  ipfs name publish '$HASH'
'

# clean-up
test_kill_ipfs_daemon
iptb stop

test_done
