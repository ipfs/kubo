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

# Please put go-ipfs specific shell functions below

# grab + output options
test "$TEST_NO_FUSE" != 1 && test_set_prereq FUSE
test "$TEST_EXPENSIVE" = 1 && test_set_prereq EXPENSIVE

if test "$TEST_VERBOSE" = 1; then
	echo '# TEST_VERBOSE='"$TEST_VERBOSE"
	echo '# TEST_NO_FUSE='"$TEST_NO_FUSE"
	echo '# TEST_EXPENSIVE='"$TEST_EXPENSIVE"
fi

# source our generic test lib
. ../../ipfs-test-lib.sh

test_cmp_repeat_10_sec() {
	for i in 1 2 3 4 5 6 7 8 9 10
	do
		test_cmp "$1" "$2" >/dev/null && return
		sleep 1
	done
	test_cmp "$1" "$2"
}

test_run_repeat_60_sec() {
	for i in 1 2 3 4 5 6
	do
		for i in 1 2 3 4 5 6 7 8 9 10
		do
			(test_eval_ "$1") && return
			sleep 1
		done
	done
	return 1 # failed
}

test_wait_output_n_lines_60_sec() {
	for i in 1 2 3 4 5 6
	do
		for i in 1 2 3 4 5 6 7 8 9 10
		do
			test $(cat "$1" | wc -l | tr -d " ") -ge $2 && return
			sleep 1
		done
	done
	actual=$(cat "$1" | wc -l | tr -d " ")
	test_fsh "expected $2 lines of output. got $actual"
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


# test_config_set helps us make sure _we really did set_ a config value.
# it sets it and then tests it. This became elaborate because ipfs config
# was setting really weird things and am not sure why.
test_config_set() {

	# grab flags (like -bool in "ipfs config -bool")
	test_cfg_flags="" # unset in case.
	test "$#" = 3 && { test_cfg_flags=$1; shift; }

	test_cfg_key=$1
	test_cfg_val=$2

	# when verbose, tell the user what config values are being set
	test_cfg_cmd="ipfs config $test_cfg_flags \"$test_cfg_key\" \"$test_cfg_val\""
	test "$TEST_VERBOSE" = 1 && echo "$test_cfg_cmd"

	# ok try setting the config key/val pair.
	ipfs config $test_cfg_flags "$test_cfg_key" "$test_cfg_val"
	echo "$test_cfg_val" >cfg_set_expected
	ipfs config "$test_cfg_key" >cfg_set_actual
	test_cmp cfg_set_expected cfg_set_actual
}

test_init_ipfs() {

	# we have a problem where initializing daemons with the same api port
	# often fails-- it hangs indefinitely. The proper solution is to make
	# ipfs pick an unused port for the api on startup, and then use that.
	# Unfortunately, ipfs doesnt yet know how to do this-- the api port
	# must be specified. Until ipfs learns how to do this, we must use
	# specific port numbers, which may still fail but less frequently
	# if we at least use different ones.

	# Using RANDOM like this is clearly wrong-- it samples with replacement
	# and it doesnt even check the port is unused. this is a trivial stop gap
	# until the proper solution is implemented.
	RANDOM=$$
	PORT_API=$((RANDOM % 3000 + 5100))
	ADDR_API="/ip4/127.0.0.1/tcp/$PORT_API"

	PORT_GWAY=$((RANDOM % 3000 + 8100))
	ADDR_GWAY="/ip4/127.0.0.1/tcp/$PORT_GWAY"

	# we set the Addresses.API config variable.
	# the cli client knows to use it, so only need to set.
	# todo: in the future, use env?

	test_expect_success "ipfs init succeeds" '
		export IPFS_PATH="$(pwd)/.go-ipfs" &&
		ipfs init -b=1024 > /dev/null
	'

	test_expect_success "prepare config -- mounting and bootstrap rm" '
		mkdir mountdir ipfs ipns &&
		test_config_set Mounts.IPFS "$(pwd)/ipfs" &&
		test_config_set Mounts.IPNS "$(pwd)/ipns" &&
		test_config_set Addresses.API "$ADDR_API" &&
		test_config_set Addresses.Gateway "$ADDR_GWAY" &&
		ipfs bootstrap rm --all ||
		test_fsh cat "\"$IPFS_PATH/config\""
	'

}

test_config_ipfs_gateway_readonly() {
	ADDR_GWAY=$1
	test_expect_success "prepare config -- gateway address" '
		test "$ADDR_GWAY" != "" &&
		test_config_set "Addresses.Gateway" "$ADDR_GWAY"
	'

	# tell the user what's going on if they messed up the call.
	if test "$#" = 0; then
		echo "#			Error: must call with an address, for example:"
		echo '#			test_config_ipfs_gateway_readonly "/ip4/0.0.0.0/tcp/5002"'
		echo '#'
	fi
}

test_config_ipfs_gateway_writable() {

	test_config_ipfs_gateway_readonly $1

	test_expect_success "prepare config -- gateway writable" '
		test_config_set -bool Gateway.Writable true ||
		test_fsh cat "\"$IPFS_PATH/config\""
	'
}

test_launch_ipfs_daemon() {

	test_expect_success "'ipfs daemon' succeeds" '
		ipfs daemon >actual_daemon 2>daemon_err &
	'

	# we say the daemon is ready when the API server is ready.
	test_expect_success "'ipfs daemon' is ready" '
		IPFS_PID=$! &&
		pollEndpoint -ep=/version -host=$ADDR_API -v -tout=1s -tries=60 2>poll_apierr > poll_apiout ||
		test_fsh cat actual_daemon || test_fsh cat daemon_err || test_fsh cat poll_apierr || test_fsh cat poll_apiout
	'

	if test "$ADDR_GWAY" != ""; then
		test_expect_success "'ipfs daemon' output includes Gateway address" '
			pollEndpoint -ep=/version -host=$ADDR_GWAY -v -tout=1s -tries=60 2>poll_gwerr > poll_gwout ||
			test_fsh cat daemon_err || test_fsh cat poll_gwerr || test_fsh cat poll_gwout
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

test_curl_resp_http_code() {
	curl -I "$1" >curl_output || {
		echo "curl error with url: '$1'"
		echo "curl output was:"
		cat curl_output
		return 1
	}
	shift &&
	RESP=$(head -1 curl_output) &&
	while test "$#" -gt 0
	do
		expr "$RESP" : "$1" >/dev/null && return
		shift
	done
	echo "curl response didn't match!"
	echo "curl response was: '$RESP'"
	echo "curl output was:"
	cat curl_output
	return 1
}
