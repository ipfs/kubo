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

test_launch_ipfs_mount

test_kill_ipfs_mount

test_expect_success "mount directories can be removed" '
	rmdir ipfs ipns
'

test_launch_ipfs_daemon

test_expect_failure "'ipfs mount' fails when no mount dir (issue #341)" '
	test_must_fail ipfs mount >actual
'

test_expect_failure "'ipfs mount' looks good when it fails (issue #341)" '
	! grep "IPFS mounted at" actual &&
	! grep "IPNS mounted at" actual
'

test_kill_ipfs_mount

test_done
