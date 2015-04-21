#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test mount command"

. lib/test-lib.sh

# if in travis CI, dont test mount (no fuse)
if ! test_have_prereq FUSE; then
	skip_all='skipping mount tests, fuse not available'

	test_done
fi

test_init_ipfs
test_launch_ipfs_daemon

#  test mount failure before mounting properly.

test_expect_success "'ipfs mount' fails when there is no mount dir" '
	tmp_ipfs_mount() { ipfs mount -f=not_ipfs -n=not_ipns >output 2>output.err; } &&
	test_must_fail tmp_ipfs_mount
'

test_expect_success "'ipfs mount' output looks good" '
	test_must_be_empty output &&
	test_should_contain "not_ipns\|not_ipfs" output.err
'

# now mount properly, and keep going
test_mount_ipfs

test_expect_success "mount directories cannot be removed while active" '
	test_must_fail rmdir ipfs ipns 2>/dev/null
'

test_kill_ipfs_daemon

test_expect_success "mount directories can be removed after shutdown" '
	rmdir ipfs ipns
'

test_done
