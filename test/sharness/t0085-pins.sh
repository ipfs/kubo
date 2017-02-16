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

test_pin_dag() {
	EXTRA_ARGS=$1

	test_expect_success "'ipfs add $EXTRA_ARGS --pin=false' 1MB file" '
		random 1048576 56 > afile &&
		HASH=`ipfs add $EXTRA_ARGS --pin=false -q afile`
	'

	test_expect_success "'ipfs pin add' file" '
		ipfs pin add --recursive=true $HASH
	'

	test_expect_success "'ipfs pin rm' file" '
		ipfs pin rm $HASH
	'

	test_expect_success "remove part of the dag" '
		PART=`ipfs refs $HASH | head -1` &&
		ipfs block rm $PART
	'

	test_expect_success "pin file, should fail" '
		test_must_fail ipfs pin add --recursive=true $HASH 2> err &&
		cat err &&
		grep -q "not found" err
	'
}

test_init_ipfs

test_pins

test_pin_dag
test_pin_dag --raw-leaves

test_launch_ipfs_daemon --offline

test_pins

test_pin_dag
test_pin_dag --raw-leaves

test_kill_ipfs_daemon

test_done
