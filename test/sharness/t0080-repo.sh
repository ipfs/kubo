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
	echo "some text" >afile &&
	HASH=`ipfs add -q afile`
'

test_expect_success "added file was pinned" '
	ipfs pin ls -type=recursive >actual &&
	grep "$HASH" actual
'

test_expect_success "'ipfs repo gc' succeeds" '
	ipfs repo gc >gc_out_actual
'

test_expect_success "'ipfs repo gc' looks good (empty)" '
	true >empty &&
	test_cmp empty gc_out_actual
'

test_expect_success "'ipfs repo gc' doesnt remove file" '
	ipfs cat "$HASH" >out &&
	test_cmp out afile
'

test_expect_success "'ipfs pin rm' succeeds" '
	ipfs pin rm -r "$HASH" >actual1
'

test_expect_success "'ipfs pin rm' output looks good" '
	echo "unpinned $HASH" >expected1 &&
	test_cmp expected1 actual1
'

test_expect_success "file no longer pinned" '
	# we expect the welcome files to show up here
	echo "$HASH_WELCOME_DOCS" >expected2 &&
	ipfs refs -r "$HASH_WELCOME_DOCS" >>expected2 &&
	sort expected2 >expected_sorted2 &&
	ipfs pin ls -type=recursive >actual2 &&
	sort actual2 >actual_sorted2 &&
	test_cmp expected_sorted2 actual_sorted2
'

test_expect_success "recursively pin afile" '
	ipfs pin add -r "$HASH"
'

test_expect_success "pinning directly should fail now" '
	echo "Error: pin: $HASH already pinned recursively" >expected3 &&
	test_must_fail ipfs pin add "$HASH" 2>actual3 &&
	test_cmp expected3 actual3
'

test_expect_success "'ipfs pin rm <hash>' should fail" '
	echo "Error: $HASH is pinned recursively" >expected4 &&
	test_must_fail ipfs pin rm "$HASH" 2>actual4 &&
	test_cmp expected4 actual4
'

test_expect_success "remove recursive pin, add direct" '
	echo "unpinned $HASH" >expected5 &&
	ipfs pin rm -r "$HASH" >actual5 &&
	test_cmp expected5 actual5 &&
	ipfs pin add "$HASH"
'

test_expect_success "remove direct pin" '
	echo "unpinned $HASH" >expected6 &&
	ipfs pin rm "$HASH" >actual6 &&
	test_cmp expected6 actual6
'

test_expect_success "'ipfs repo gc' removes file" '
	echo "removed $HASH" >expected7 &&
	ipfs repo gc >actual7 &&
	test_cmp expected7 actual7
'

test_expect_success "'ipfs refs local' no longer shows file" '
	echo QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn >expected8 &&
	echo "$HASH_WELCOME_DOCS" >>expected8 &&
	ipfs refs -r "$HASH_WELCOME_DOCS" >>expected8 &&
	sort expected8 >expected_sorted8 &&
	ipfs refs local >actual8 &&
	sort actual8 >actual_sorted8 &&
	test_cmp expected_sorted8 actual_sorted8
'

test_expect_success "adding multiblock random file succeeds" '
	random 1000000 >multiblock &&
	MBLOCKHASH=`ipfs add -q multiblock`
'

test_expect_success "'ipfs pin ls -type=indirect' is correct" '
	ipfs refs "$MBLOCKHASH" >refsout &&
	ipfs refs -r "$HASH_WELCOME_DOCS" >>refsout &&
	sort refsout >refsout_sorted &&
	ipfs pin ls -type=indirect >indirectpins &&
	sort indirectpins >indirectpins_sorted &&
	test_cmp refsout_sorted indirectpins_sorted
'

test_expect_success "pin something directly" '
	echo "ipfs is so awesome" >awesome &&
	DIRECTPIN=`ipfs add -q awesome` &&
	echo "unpinned $DIRECTPIN" >expected9 &&
	ipfs pin rm -r "$DIRECTPIN" >actual9 &&
	test_cmp expected9 actual9  &&

	echo "pinned $DIRECTPIN directly" >expected10 &&
	ipfs pin add "$DIRECTPIN" >actual10 &&
	test_cmp expected10 actual10
'

test_expect_success "'ipfs pin ls -type=direct' is correct" '
	echo "$DIRECTPIN" >directpinexpected &&
	echo QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn >>directpinexpected &&
	sort directpinexpected >dp_exp_sorted &&
	ipfs pin ls -type=direct >directpinout &&
	sort directpinout >dp_out_sorted &&
	test_cmp dp_exp_sorted dp_out_sorted
'

test_expect_success "'ipfs pin ls -type=recursive' is correct" '
	echo "$MBLOCKHASH" >rp_expected &&
	echo "$HASH_WELCOME_DOCS" >>rp_expected &&
	ipfs refs -r "$HASH_WELCOME_DOCS" >>rp_expected &&
	sort rp_expected >rp_exp_sorted &&
	ipfs pin ls -type=recursive >rp_actual &&
	sort rp_actual >rp_act_sorted &&
	test_cmp rp_exp_sorted rp_act_sorted
'

test_expect_success "'ipfs pin ls -type=all' is correct" '
	cat directpinout >allpins &&
	cat rp_actual >>allpins &&
	cat indirectpins >>allpins &&
	sort allpins >allpins_sorted &&
	ipfs pin ls -type=all >actual_allpins &&
	sort actual_allpins >actual_allpins_sorted &&
	test_cmp allpins_sorted actual_allpins_sorted
'

test_kill_ipfs_daemon

test_done
