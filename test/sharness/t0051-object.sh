#!/bin/sh
#
# Copyright (c) 2015 Henry Bubert
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test object command"

. lib/test-lib.sh

test_init_ipfs

test_object_cmd() {

	test_expect_success "'ipfs add testData' succeeds" '
		printf "Hello Mars" >expected_in &&
		ipfs add expected_in >actual_Addout
	'
	
	test_expect_success "'ipfs add testData' output looks good" '
		HASH="QmWkHFpYBZ9mpPRreRbMhhYWXfUhBAue3JkbbpFqwowSRb" &&
		echo "added $HASH expected_in" >expected_Addout &&
		test_cmp expected_Addout actual_Addout
	'
	
	test_expect_success "'ipfs object get' succeeds" '
		ipfs object get $HASH >actual_getOut
	'
	
	test_expect_success "'ipfs object get' output looks good" '
		test_cmp ../t0051-object-data/expected_getOut actual_getOut
	'
	
	test_expect_success "'ipfs object stat' succeeds" '
		ipfs object stat $HASH >actual_stat
	'
	
	test_expect_success "'ipfs object get' output looks good" '
		echo "NumLinks: 0" > expected_stat &&
		echo "BlockSize: 18" >> expected_stat &&
		echo "LinksSize: 2" >> expected_stat &&
		echo "DataSize: 16" >> expected_stat &&
		echo "CumulativeSize: 18" >> expected_stat &&
		test_cmp expected_stat actual_stat
	'
	
	test_expect_success "'ipfs object put file.json' succeeds" '
		ipfs object put  ../t0051-object-data/testPut.json > actual_putOut
	'
	
	test_expect_success "'ipfs object put file.json' output looks good" '
		HASH="QmUTSAdDi2xsNkDtLqjFgQDMEn5di3Ab9eqbrt4gaiNbUD" &&
		printf "added $HASH" > expected_putOut &&
		test_cmp expected_putOut actual_putOut
	'
	
	test_expect_success "'ipfs object put file.pb' succeeds" '
		ipfs object put --inputenc=protobuf ../t0051-object-data/testPut.pb > actual_putOut
	'
	
	test_expect_success "'ipfs object put file.pb' output looks good" '
		HASH="QmUTSAdDi2xsNkDtLqjFgQDMEn5di3Ab9eqbrt4gaiNbUD" &&
		printf "added $HASH" > expected_putOut &&
		test_cmp expected_putOut actual_putOut
	'
	
	test_expect_success "'ipfs object put' from stdin succeeds" '
		cat ../t0051-object-data/testPut.json | ipfs object put > actual_putStdinOut
	'
	
	test_expect_success "'ipfs object put' from stdin output looks good" '
		HASH="QmUTSAdDi2xsNkDtLqjFgQDMEn5di3Ab9eqbrt4gaiNbUD" &&
		printf "added $HASH" > expected_putStdinOut &&
		test_cmp expected_putStdinOut actual_putStdinOut
	'
	
	test_expect_success "'ipfs object put' from stdin (pb) succeeds" '
		cat ../t0051-object-data/testPut.pb | ipfs object put --inputenc=protobuf > actual_putPbStdinOut
	'
	
	test_expect_success "'ipfs object put' from stdin (pb) output looks good" '
		HASH="QmUTSAdDi2xsNkDtLqjFgQDMEn5di3Ab9eqbrt4gaiNbUD" &&
		printf "added $HASH" > expected_putStdinOut &&
		test_cmp expected_putStdinOut actual_putPbStdinOut
	'
	
	test_expect_success "'ipfs object put broken.json' should fail" '
		test_expect_code 1 ipfs object put ../t0051-object-data/brokenPut.json 2>actual_putBrokenErr >actual_putBroken
	'
	
	test_expect_success "'ipfs object put broken.hjson' output looks good" '
		touch expected_putBroken &&
		printf "Error: no data or links in this node\n" > expected_putBrokenErr &&
		test_cmp expected_putBroken actual_putBroken &&
		test_cmp expected_putBrokenErr actual_putBrokenErr
	'

	test_expect_success "'ipfs object patch' should work" '
		EMPTY_DIR=$(ipfs object new unixfs-dir) &&
		OUTPUT=$(ipfs object patch $EMPTY_DIR add-link foo $EMPTY_DIR)
	'

	test_expect_success "should have created dir within a dir" '
		ipfs ls $OUTPUT > patched_output
	'

	test_expect_success "output looks good" '
		echo "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn 4 foo/ " > patched_exp &&
		test_cmp patched_exp patched_output
	'

	test_expect_success "can remove the directory" '
		ipfs object patch $OUTPUT rm-link foo > rmlink_output
	'

	test_expect_success "output should be empty" '
		echo QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn > rmlink_exp &&
		test_cmp rmlink_exp rmlink_output
	'
}

# should work offline
test_object_cmd

# should work online
test_launch_ipfs_daemon
test_object_cmd
test_kill_ipfs_daemon

test_done
