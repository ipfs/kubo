# Test framework for go-ipfs
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#
# We are using sharness (https://github.com/mlafeldt/sharness)
# which was extracted from the Git test framework.

# use the ipfs tool to test against

# add current directory to path, for ipfs tool.
PATH=$(pwd):${PATH}

# assert the `ipfs` we're using is the right one.
if test `which ipfs` != $(pwd)/ipfs; then
	echo >&2 "Cannot find the tests' local ipfs tool."
	echo >&2 "Please check test and ipfs tool installation."
	exit 1
fi

SHARNESS_LIB="./sharness.sh"

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

test_launch_ipfs_mount() {

	test_expect_success "ipfs init succeeds" '
		export IPFS_DIR="$(pwd)/.go-ipfs" &&
		ipfs init -b=2048
	'

	test_expect_success "prepare config" '
		mkdir mountdir ipfs ipns &&
		ipfs config Mounts.IPFS "$(pwd)/ipfs" &&
		ipfs config Mounts.IPNS "$(pwd)/ipns"
	'

	test_expect_success FUSE "ipfs mount succeeds" '
		ipfs mount mountdir >actual &
	'

	test_expect_success FUSE "ipfs mount output looks good" '
		IPFS_PID=$! &&
		echo "mounting ipfs at $(pwd)/ipfs" >expected &&
		echo "mounting ipns at $(pwd)/ipns" >>expected &&
		test_cmp_repeat_10_sec expected actual
	'
}

test_kill_ipfs_mount() {

	test_expect_success FUSE "ipfs mount is still running" '
		kill -0 $IPFS_PID
	'

	test_expect_success FUSE "ipfs mount can be killed" '
		kill $IPFS_PID &&
		sleep 1 &&
		! kill -0 $IPFS_PID 2>/dev/null
	'
}
