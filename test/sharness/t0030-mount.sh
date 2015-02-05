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

# run this mount failure before mounting properly.

test_expect_failure "'ipfs mount' fails when no mount dir (issue #341)" '
	test_must_fail ipfs mount -f=not_ipfs -n=not_ipns >actual
'

test_expect_failure "'ipfs mount' looks good when it fails (issue #341)" '
	! grep "IPFS mounted at: $(pwd)/ipfs" actual >/dev/null &&
	! grep "IPNS mounted at: $(pwd)/ipns" actual >/dev/null ||
	test_fsh cat actual
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
