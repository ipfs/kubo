#!/bin/sh
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test dag command"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "make a few test files" '
	echo "foo" > file1 &&
	echo "bar" > file2 &&
	echo "baz" > file3 &&
	echo "qux" > file4 &&
	HASH1=$(ipfs add -q file1) &&
	HASH2=$(ipfs add -q file2) &&
	HASH3=$(ipfs add -q file3) &&
	HASH4=$(ipfs add -q file4)
'

test_expect_success "make an ipld object in json" '
	printf "{\"hello\":\"world\",\"cats\":[{\"/\":\"%s\"},{\"water\":{\"/\":\"%s\"}}],\"magic\":{\"/\":\"%s\"}}" $HASH1 $HASH2 $HASH3 > ipld_object
'

test_dag_cmd() {
	test_expect_success "can add an ipld object" '
		IPLDHASH=$(cat ipld_object | ipfs dag put)
	'

	test_expect_success "output looks correct" '
		EXPHASH="zdpuApvChR5xM7ttbQmpmtna7wcShHi4gPyxUcWbB7nh8K7cN"
		test $EXPHASH = $IPLDHASH
	'

	test_expect_success "various path traversals work" '
		ipfs cat $IPLDHASH/cats/0 > out1 &&
		ipfs cat $IPLDHASH/cats/1/water > out2 &&
		ipfs cat $IPLDHASH/magic > out3
	'

	test_expect_success "outputs look correct" '
		test_cmp file1 out1 &&
		test_cmp file2 out2 &&
		test_cmp file3 out3
	'
}

# should work offline
test_dag_cmd

# should work online
test_launch_ipfs_daemon
test_dag_cmd
test_kill_ipfs_daemon

test_done
