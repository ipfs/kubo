#!/bin/sh
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs repo operations"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

test_expect_success "'ipfs add afile' succeeds" '
	echo "some text" >afile
	HASH=`ipfs add -q afile`
	printf "$HASH" >hashfile
'

test_expect_success "added file was pinned" '
	ipfs pin ls -type=recursive | grep $HASH
'

test_expect_success "'ipfs repo gc' succeeds" '
	ipfs repo gc >gc_out_actual
'

test_expect_success "'ipfs repo gc' looks good (empty)" '
	printf "" >empty
	test_cmp empty gc_out_actual
'

test_expect_success "'ipfs repo gc' doesnt remove file" '
	ipfs cat $HASH >out
	test_cmp out afile
'

test_expect_success "'ipfs pin rm' succeeds" '
	echo unpinned $HASH >expected1
	ipfs pin rm -r $HASH >actual1
	test_cmp expected1 actual1
'

test_expect_success "file no longer pinned" '
	# we expect the welcome file to show up here
	echo QmTTFXiXoixwT53tcGPu419udsHEHYu6AHrQC8HAKdJYaZ >expected2
	ipfs pin ls -type=recursive >actual2
	test_cmp expected2 actual2
'

test_expect_success "recursively pin afile" '
	ipfs pin add -r $HASH
'

test_expect_success "pinning directly should fail now" '
	echo Error: pin: $HASH already pinned recursively >expected3
	ipfs pin add $HASH 2>actual3
	test_cmp expected3 actual3
'

test_expect_success "'ipfs pin rm <hash>' should fail" '
	echo Error: $HASH is pinned recursively >expected4
	ipfs pin rm $HASH 2>actual4
	test_cmp expected4 actual4
'

test_expect_success "remove recursive pin, add direct" '
	echo unpinned $HASH >expected5
	ipfs pin rm -r $HASH >actual5
	test_cmp expected5 actual5
	ipfs pin add $HASH
'

test_expect_success "remove direct pin" '
	echo unpinned $HASH >expected6
	ipfs pin rm $HASH >actual6
	test_cmp expected6 actual6
'

test_expect_success "'ipfs repo gc' removes file" '
	echo removed $HASH >expected7
	ipfs repo gc >actual7
	test_cmp expected7 actual7
'

test_expect_success "'ipfs refs local' no longer shows file" '
	echo QmTTFXiXoixwT53tcGPu419udsHEHYu6AHrQC8HAKdJYaZ >expected8
	echo QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn >>expected8
	ipfs refs local >actual8
	test_cmp expected8 actual8
'

test_expect_success "adding multiblock random file succeeds" '
	random 1000000 >multiblock
	MBLOCKHASH=`ipfs add -q multiblock`
'

test_expect_success "'ipfs pin ls -type=indirect' is correct" '
	ipfs refs $MBLOCKHASH | sort >refsout
	ipfs pin ls -type=indirect | sort >indirectpins
	test_cmp refsout indirectpins
'

test_expect_success "pin something directly" '
	echo "ipfs is so awesome" >awesome
	DIRECTPIN=`ipfs add -q awesome`
	echo unpinned $DIRECTPIN >expected9
	ipfs pin rm -r $DIRECTPIN >actual9
	test_cmp expected9 actual9

	echo pinned $DIRECTPIN directly >expected10
	ipfs pin add $DIRECTPIN >actual10
	test_cmp expected10 actual10
'

test_expect_success "'ipfs pin ls -type=direct' is correct" '
	echo $DIRECTPIN >directpinexpected
	echo QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn >>directpinexpected
	cat directpinexpected | sort >dp_exp_sorted
	ipfs pin ls -type=direct | sort >directpinout
	test_cmp dp_exp_sorted directpinout
'

test_expect_success "'ipfs pin ls -type=recursive' is correct" '
	echo $MBLOCKHASH >rp_expected
	echo QmTTFXiXoixwT53tcGPu419udsHEHYu6AHrQC8HAKdJYaZ >>rp_expected
	cat rp_expected | sort >rp_exp_sorted
	ipfs pin ls -type=recursive | sort >rp_actual
	test_cmp rp_exp_sorted rp_actual
'

test_expect_success "'ipfs pin ls -type=all' is correct" '
	cat directpinout >allpins
	cat rp_actual >>allpins
	cat indirectpins >>allpins
	cat allpins | sort >allpins_sorted
	ipfs pin ls -type=all | sort >actual_allpins
	test_cmp allpins_sorted actual_allpins
'

test_kill_ipfs_daemon

test_done
