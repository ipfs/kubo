#!/bin/sh
#
# Copyright (c) Jakub Sztandera
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs blockstore repo read check."

. lib/test-lib.sh

rm -rf "$IPF_PATH/*"

test_init_ipfs


H_BLOCK1=$(echo "Block 1" | ipfs add -q)
H_BLOCK2=$(echo "Block 2" | ipfs add -q)

BS_BLOCK1="1220f18e/1220f18e07ebc69997909358f28b9d2c327eb032b0afab6bbc7fd7f399a7b7590be4.data"
BS_BLOCK2="1220dc58/1220dc582e51f1f98b1f2d1c1baaa9f7b11602239ed42fbdf8f52d67e63cc03df12a.data"


test_expect_success 'blocks are swapped' '
	ipfs cat $H_BLOCK2 > noswap &&
	cp -f "$IPFS_PATH/blocks/$BS_BLOCK1" "$IPFS_PATH/blocks/$BS_BLOCK2" &&
	ipfs cat $H_BLOCK2 > swap &&
	test_must_fail test_cmp noswap swap
'

ipfs config --bool Datastore.HashOnRead true

test_expect_success 'getting modified block fails' '
	(test_must_fail ipfs cat $H_BLOCK2 2> err_msg) &&
	grep "block in storage has different hash than requested" err_msg
'

test_done
