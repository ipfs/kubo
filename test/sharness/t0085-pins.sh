#!/bin/sh
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs pinning operations"

. lib/test-lib.sh


test_pins() {
	test_expect_success "create some hashes" '
		HASH_A=$(echo "A" | ipfs add -q --pin=false) &&
		HASH_B=$(echo "B" | ipfs add -q --pin=false) &&
		HASH_C=$(echo "C" | ipfs add -q --pin=false) &&
		HASH_D=$(echo "D" | ipfs add -q --pin=false) &&
		HASH_E=$(echo "E" | ipfs add -q --pin=false) &&
		HASH_F=$(echo "F" | ipfs add -q --pin=false) &&
		HASH_G=$(echo "G" | ipfs add -q --pin=false)
	'

	test_expect_success "put all those hashes in a file" '
		echo $HASH_A > hashes &&
		echo $HASH_B >> hashes &&
		echo $HASH_C >> hashes &&
		echo $HASH_D >> hashes &&
		echo $HASH_E >> hashes &&
		echo $HASH_F >> hashes &&
		echo $HASH_G >> hashes
	'

	test_expect_success "pin those hashes via stdin" '
		cat hashes | ipfs pin add
	'

	test_expect_success "unpin those hashes" '
		cat hashes | ipfs pin rm
	'
}

test_init_ipfs

test_pins

test_launch_ipfs_daemon --offline

test_pins

test_kill_ipfs_daemon

test_done
