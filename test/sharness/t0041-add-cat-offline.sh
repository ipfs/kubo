#!/bin/sh
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add and cat commands"

. lib/test-lib.sh

client_err() {
    printf "$@\n\nUse 'ipfs add --help' for information about this command\n"
}

test_init_ipfs

test_expect_success "ipfs add file succeeds" '
	echo "some content" > afile &&
	HASH=$(ipfs add -q afile)
'

test_expect_success "ipfs add output looks good" '
	echo Qmb1EXrDyKhNWfvLPYK4do3M9nU7BuLAcbqBir6aUrDsRY > expected &&
	echo $HASH > actual &&
	test_cmp expected actual
'

test_expect_success "ipfs add --only-hash succeeds" '
	ipfs add -q --only-hash afile > ho_output
'

test_expect_success "ipfs add --only-hash output looks good" '
	test_cmp expected ho_output
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

test_expect_success "ipfs add --only-hash succeeds" '
	echo "unknown content for only-hash" | ipfs add --only-hash -q > oh_hash
'

test_expect_success "ipfs cat file fails" '
	test_must_fail ipfs cat $(cat oh_hash)
'

test_expect_success "useful error message when adding a named pipe" '
	mkfifo named-pipe &&
	test_expect_code 1 ipfs add named-pipe 2>actual &&
    client_err "Error: Unrecognized file type for named-pipe: $(generic_stat named-pipe)" >expected &&
	test_cmp expected actual
'

test_expect_success "useful error message when recursively adding a named pipe" '
	mkdir named-pipe-dir &&
	mkfifo named-pipe-dir/named-pipe &&
	test_expect_code 1 ipfs add -r named-pipe-dir 2>actual &&
    printf "Error: Unrecognized file type for named-pipe-dir/named-pipe: $(generic_stat named-pipe-dir/named-pipe)\n" >expected &&
	test_cmp expected actual
'

test_done
