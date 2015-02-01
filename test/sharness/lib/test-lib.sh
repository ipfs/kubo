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

# set sharness verbosity. we set the env var directly as
# it's too late to pass in --verbose, and --verbose is harder
# to pass through in some cases.
test "$TEST_VERBOSE" = 1 && verbose=t

# assert the `ipfs` we're using is the right one.
if test `which ipfs` != $(pwd)/bin/ipfs; then
	echo >&2 "Cannot find the tests' local ipfs tool."
	echo >&2 "Please check test and ipfs tool installation."
	exit 1
fi


# source the common hashes first.
. lib/test-lib-hashes.sh


SHARNESS_LIB="lib/sharness/sharness.sh"

. "$SHARNESS_LIB" || {
	echo >&2 "Cannot source: $SHARNESS_LIB"
	echo >&2 "Please check Sharness installation."
	exit 1
}

# overriding testcmp to make it use fsh (to see it better in output)
# have to do it twice so the first diff output doesnt show unless it's
# broken.
test_cmp() {
	diff -q "$@" >/dev/null || fsh diff -u "$@"
}

# Please put go-ipfs specific shell functions below

test "$TEST_NO_FUSE" != 1 && test_set_prereq FUSE
test "$TEST_EXPENSIVE" = 1 && test_set_prereq EXPENSIVE

test_cmp_repeat_10_sec() {
	for i in 1 2 3 4 5 6 7 8 9 10
	do
		test_cmp "$1" "$2" >/dev/null && return
		sleep 1
	done
	test_cmp "$1" "$2"
}

test_run_repeat_10_sec() {
	for i in 1 2 3 4 5 6 7 8 9 10
	do
		(test_eval_ "$1") && return
		sleep 1
	done
	return 1 # failed
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

test_wait_open_tcp_port_10_sec() {
	for i in 1 2 3 4 5 6 7 8 9 10; do
		# this is not a perfect check, but it's portable.
		# cant count on ss. not installed everywhere.
		# cant count on netstat using : or . as port delim. differ across platforms.
		echo $(netstat -aln | egrep "^tcp.*LISTEN" | egrep "[.:]$1" | wc -l) -gt 0
		if [ $(netstat -aln | egrep "^tcp.*LISTEN" | egrep "[.:]$1" | wc -l) -gt 0 ]; then
			return 0
		fi
		sleep 1
	done
	return 1
}

test_init_ipfs() {

	test_expect_success "ipfs init succeeds" '
		export IPFS_PATH="$(pwd)/.go-ipfs" &&
		ipfs init -b=1024 > /dev/null
	'

	test_expect_success "prepare config" '
		mkdir mountdir ipfs ipns &&
		ipfs config Mounts.IPFS "$(pwd)/ipfs" &&
		ipfs config Mounts.IPNS "$(pwd)/ipns" &&
		ipfs bootstrap rm --all
	'

}

test_config_ipfs_gateway_readonly() {
	test_expect_success "prepare config -- gateway readonly" '
	  ipfs config Addresses.Gateway /ip4/0.0.0.0/tcp/5002
	'
}

test_config_ipfs_gateway_writable() {
	test_expect_success "prepare config -- gateway writable" '
	  ipfs config Addresses.Gateway /ip4/0.0.0.0/tcp/5002 &&
	  ipfs config -bool Gateway.Writable true
	'
}

test_launch_ipfs_daemon() {

	test_expect_success "'ipfs daemon' succeeds" '
		ipfs daemon >actual_daemon 2>daemon_err &
	'

	# we say the daemon is ready when the API server is ready.
	# and we make sure there are no errors
	test_expect_success "'ipfs daemon' is ready" '
		IPFS_PID=$! &&
		test_run_repeat_10_sec "cat actual_daemon | grep \"API server listening on\"" &&
		printf "" >empty && test_cmp daemon_err empty ||
		fsh cat actual_daemon || fsh cat daemon_err
	'

	ADDR_API="/ip4/127.0.0.1/tcp/5001"
	test_expect_success "'ipfs daemon' output includes API address" '
		cat actual_daemon | grep "API server listening on $ADDR_API" ||
		fsh cat actual_daemon ||
		fsh "cat actual_daemon | grep \"API server listening on $ADDR_API\""
	'

	ADDR_GWAY=`ipfs config Addresses.Gateway`
	if test "$ADDR_GWAY" != ""; then
		test_expect_success "'ipfs daemon' output includes Gateway address" '
			cat actual_daemon | grep "Gateway server listening on $ADDR_GWAY" ||
			fsh cat actual_daemon ||
			fsh "cat actual_daemon | grep \"Gateway server listening on $ADDR_GWAY\""
		'
	fi
}

test_mount_ipfs() {

	# make sure stuff is unmounted first.
	test_expect_success FUSE "'ipfs mount' succeeds" '
		umount $(pwd)/ipfs || true &&
		umount $(pwd)/ipns || true &&
		ipfs mount >actual
	'

	test_expect_success FUSE "'ipfs mount' output looks good" '
		echo "IPFS mounted at: $(pwd)/ipfs" >expected &&
		echo "IPNS mounted at: $(pwd)/ipns" >>expected &&
		test_cmp expected actual
	'

}

test_launch_ipfs_daemon_and_mount() {

	test_init_ipfs
	test_launch_ipfs_daemon
	test_mount_ipfs

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

test_kill_ipfs_daemon() {

	test_expect_success "'ipfs daemon' is still running" '
		kill -0 $IPFS_PID
	'

	test_expect_success "'ipfs daemon' can be killed" '
		test_kill_repeat_10_sec $IPFS_PID
	'
}
