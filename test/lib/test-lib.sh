# Test framework for go-ipfs
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#
# We are using sharness (https://github.com/mlafeldt/sharness)
# which was extracted from the Git test framework.

# use the ipfs tool to test against

# add current directory to path, for ipfs tool.
PATH=$(pwd)/bin:${PATH}

# assert the `ipfs` we're using is the right one.
if test `which ipfs` != $(pwd)/bin/ipfs; then
	echo >&2 "Cannot find the tests' local ipfs tool."
	echo >&2 "Please check test and ipfs tool installation."
	exit 1
fi

SHARNESS_LIB="lib/sharness/sharness.sh"

. "$SHARNESS_LIB" || {
	echo >&2 "Cannot source: $SHARNESS_LIB"
	echo >&2 "Please check Sharness installation."
	exit 1
}

# Please put go-ipfs specific shell functions below

test "$TEST_NO_FUSE" != 1 && test_set_prereq FUSE

test_cmp_repeat_10_sec() {
	for i in 1 2 3 4 5 6 7 8 9 10
	do
		test_cmp "$1" "$2" && return
		sleep 1
	done
	test_cmp "$1" "$2"
}

test_wait_output_n_lines_60_sec() {
	echo "$2" >expected_waitn
	for i in 1 2 3 4 5 6 7 8 9 10
	do
		cat "$1" | wc -l | tr -d " " >actual_waitn
		test_cmp "expected_waitn" "actual_waitn" && return
		sleep 2
	done
	cat "$1" | wc -l | tr -d " " >actual_waitn
	test_cmp "expected_waitn" "actual_waitn"
}

test_launch_ipfs_mount() {

	test_expect_success "ipfs init succeeds" '
		export IPFS_DIR="$(pwd)/.go-ipfs" &&
		ipfs init -b=1024
	'

	test_expect_success "prepare config" '
		mkdir mountdir ipfs ipns &&
		ipfs config Mounts.IPFS "$(pwd)/ipfs" &&
		ipfs config Mounts.IPNS "$(pwd)/ipns"
	'

	test_expect_success FUSE "'ipfs daemon' succeeds" '
		ipfs daemon >actual &
	'

	test_expect_success FUSE "'ipfs daemon' output looks good" '
		IPFS_PID=$! &&
		echo "daemon listening on /ip4/127.0.0.1/tcp/5001" >expected &&
		test_cmp_repeat_10_sec expected actual
	'

	test_expect_success FUSE "'ipfs mount' succeeds" '
		ipfs mount >actual
	'

	test_expect_success FUSE "'ipfs mount' output looks good" '
		echo "IPFS mounted at: $(pwd)/ipfs" >expected &&
		echo "IPNS mounted at: $(pwd)/ipns" >>expected &&
		test_cmp expected actual
	'
}

test_kill_repeat_10_sec() {
	for i in 1 2 3 4 5 6 7 8 9 10
	do
		kill $1
		sleep 1
		! kill -0 $1 2>/dev/null && return
	done
	! kill -0 $1 2>/dev/null
}

test_kill_ipfs_mount() {

	test_expect_success FUSE "'ipfs daemon' is still running" '
		kill -0 $IPFS_PID
	'

	test_expect_success FUSE "'ipfs daemon' can be killed" '
		test_kill_repeat_10_sec $IPFS_PID
	'
}
