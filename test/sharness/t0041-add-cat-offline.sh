#!/bin/sh
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add and cat commands"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "ipfs add file succeeds" '
	echo "some content" > afile &&
	HASH=$(ipfs add -q afile)
'

test_expect_success "ipfs cat file suceeds" '
	ipfs cat $HASH > out_1
'

test_expect_success "output looks good" '
	test_cmp afile out_1
'

test_expect_success "ipfs cat /ipfs/file succeeds" '
	ipfs cat /ipfs/$HASH > out_2
'

test_expect_success "output looks good" '
	test_cmp afile out_2
'

test_expect_success "useful error message when adding a named pipe" '
	mkfifo named-pipe
	test_expect_code 1 ipfs add named-pipe 2>named-pipe-error
	echo "Error: \`named-pipe\` is an unknown type" >named-pipe-error-expected
	test_cmp named-pipe-error-expected named-pipe-error
'

test_expect_success "useful error message when recursively adding a named pipe" '
	mkdir named-pipe-dir
	mkfifo named-pipe-dir/named-pipe
	test_expect_code 1 ipfs add -r named-pipe-dir 2>named-pipe-dir-error
	echo "Error: \`named-pipe-dir/named-pipe\` is an unknown type" >named-pipe-dir-error-expected
	test_cmp named-pipe-dir-error-expected named-pipe-dir-error
'

test_done
