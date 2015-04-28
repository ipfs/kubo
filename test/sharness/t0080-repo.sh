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
	ipfs pin ls --type=recursive >actual &&
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
	echo QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn >> expected2 &&
	ipfs pin ls --type=recursive --quiet >actual2 &&
	test_sort_cmp expected2 actual2
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

# TODO: there seems to be a serious bug with leveldb not returning a key.
test_expect_failure "'ipfs refs local' no longer shows file" '
	echo QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn >expected8 &&
	echo "$HASH_WELCOME_DOCS" >>expected8 &&
	ipfs refs -r "$HASH_WELCOME_DOCS" >>expected8 &&
	ipfs refs local >actual8 &&
	test_sort_cmp expected8 actual8
'

test_expect_success "adding multiblock random file succeeds" '
	random 1000000 >multiblock &&
	MBLOCKHASH=`ipfs add -q multiblock`
'

test_expect_success "'ipfs pin ls --type=indirect' is correct" '
	ipfs refs "$MBLOCKHASH" >refsout &&
	ipfs refs -r "$HASH_WELCOME_DOCS" >>refsout &&
	sed -i="" "s/\(.*\)/\1 indirect/g" refsout &&
	ipfs pin ls --type=indirect >indirectpins &&
	test_sort_cmp refsout indirectpins
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

test_expect_success "'ipfs pin ls --type=direct' is correct" '
	echo "$DIRECTPIN direct" >directpinexpected &&
	ipfs pin ls --type=direct >directpinout &&
	test_sort_cmp directpinexpected directpinout
'

test_expect_success "'ipfs pin ls --type=recursive' is correct" '
	echo "$MBLOCKHASH" >rp_expected &&
	echo "$HASH_WELCOME_DOCS" >>rp_expected &&
	echo QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn >>rp_expected &&
	ipfs refs -r "$HASH_WELCOME_DOCS" >>rp_expected &&
	sed -i="" "s/\(.*\)/\1 recursive/g" rp_expected &&
	ipfs pin ls --type=recursive >rp_actual &&
	test_sort_cmp rp_expected rp_actual
'

test_expect_success "'ipfs pin ls --type=all --quiet' is correct" '
	cat directpinout >allpins &&
	cat rp_actual >>allpins &&
	cat indirectpins >>allpins &&
	cut -f1 -d " " allpins | sort | uniq >> allpins_uniq_hashes &&
	ipfs pin ls --type=all --quiet >actual_allpins &&
	test_sort_cmp allpins_uniq_hashes actual_allpins
'

test_kill_ipfs_daemon

test_done
