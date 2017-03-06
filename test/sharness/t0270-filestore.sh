#!/bin/sh
#
# Copyright (c) 2017 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test out the filestore nocopy functionality"

. lib/test-lib.sh


test_expect_success "create a dataset" '
	random-files -seed=483 -depth=3 -dirs=4 -files=6 -filesize=1000000 somedir
'

EXPHASH="QmW4JLyeTxEWGwa4mkE9mHzdtAkyhMX2ToGFEKZNjCiJud"

get_repo_size() {
	disk_usage "$IPFS_PATH"
}

assert_repo_size_less_than() {
	expval="$1"

	test_expect_success "check repo size" '
		test "$(get_repo_size)"	-lt "$expval" ||
		test_fsh get_repo_size
	'
}

assert_repo_size_greater_than() {
	expval="$1"

	test_expect_success "check repo size" '
		test "$(get_repo_size)"	-gt "$expval" ||
			test_fsh get_repo_size
	'
}

test_filestore_adds() {
	test_expect_success "nocopy add succeeds" '
		HASH=$(ipfs add --raw-leaves --nocopy -r -q somedir | tail -n1)
	'

	test_expect_success "nocopy add has right hash" '
		test "$HASH" = "$EXPHASH"
	'

	assert_repo_size_less_than 1000000

	test_expect_success "normal add with fscache doesnt duplicate data" '
		HASH2=$(ipfs add --raw-leaves --fscache -r -q somedir | tail -n1)
	'

	assert_repo_size_less_than 1000000

	test_expect_success "normal add without fscache duplicates data" '
		HASH2=$(ipfs add --raw-leaves -r -q somedir | tail -n1)
	'

	assert_repo_size_greater_than 1000000
}

init_ipfs_filestore() {
	test_expect_success "clean up old node" '
		rm -rf "$IPFS_PATH" mountdir ipfs ipns
	'

	test_init_ipfs

	test_expect_success "nocopy add errors and has right message" '
		test_must_fail ipfs add --nocopy -r somedir 2> add_out &&
			grep "filestore is not enabled" add_out
	'


	test_expect_success "enable filestore config setting" '
		ipfs config --json Experimental.FilestoreEnabled true
	'
}

init_ipfs_filestore

test_filestore_adds

echo "WORKING DIR"
echo "IPFS PATH = " $IPFS_PATH
pwd


init_ipfs_filestore

test_launch_ipfs_daemon

test_filestore_adds

test_kill_ipfs_daemon

test_done
