#!/usr/bin/env bash
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs repo operations"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon_without_network

test_expect_success "'ipfs repo gc' succeeds" '
  ipfs repo gc >gc_out_actual
'

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

test_expect_success "'ipfs repo gc' looks good (patch root)" '
  grep -v "removed $HASH" gc_out_actual
'

test_expect_success "'ipfs repo gc' doesn't remove file" '
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

test_expect_success "ipfs repo gc fully reverse ipfs add (part 1)" '
  ipfs repo gc &&
  random 100000 41 >gcfile &&
  find "$IPFS_PATH/blocks" -type f -name "*.data" | sort -u > expected_blocks &&
  hash=$(ipfs add -q gcfile) &&
  ipfs pin rm -r $hash &&
  ipfs repo gc
'

test_kill_ipfs_daemon

test_expect_success "ipfs repo gc fully reverse ipfs add (part 2)" '
  find "$IPFS_PATH/blocks" -type f -name "*.data" | sort -u > actual_blocks &&
  test_cmp expected_blocks actual_blocks
'

test_launch_ipfs_daemon_without_network

test_expect_success "file no longer pinned" '
  ipfs pin ls --type=recursive --quiet >actual2 &&
  test_expect_code 1 grep $HASH actual2
'

test_expect_success "recursively pin afile(default action)" '
  HASH=`ipfs add -q afile` &&
  ipfs pin add "$HASH"
'

test_expect_success "recursively pin rm afile (default action)" '
  ipfs pin rm "$HASH"
'

test_expect_success "recursively pin afile" '
  ipfs pin add -r "$HASH"
'

test_expect_success "pinning directly should fail now" '
  echo "Error: pin: $HASH already pinned recursively" >expected3 &&
  test_must_fail ipfs pin add -r=false "$HASH" 2>actual3 &&
  test_cmp expected3 actual3
'

test_expect_success "'ipfs pin rm -r=false <hash>' should fail" '
  echo "Error: $HASH is pinned recursively" >expected4
  test_must_fail ipfs pin rm -r=false "$HASH" 2>actual4 &&
  test_cmp expected4 actual4
'

test_expect_success "remove recursive pin, add direct" '
  echo "unpinned $HASH" >expected5 &&
  ipfs pin rm -r "$HASH" >actual5 &&
  test_cmp expected5 actual5 &&
  ipfs pin add -r=false "$HASH"
'

test_expect_success "remove direct pin" '
  echo "unpinned $HASH" >expected6 &&
  ipfs pin rm "$HASH" >actual6 &&
  test_cmp expected6 actual6
'

test_expect_success "'ipfs repo gc' removes file" '
  ipfs block stat $HASH &&
  ipfs repo gc &&
  test_must_fail ipfs block stat $HASH
'

# Convert all to a base32-multihash as refs local outputs cidv1 raw
# Technically converting refs local output would suffice, but this is more
# future proof if we ever switch to adding the files with cid-version 1.
test_expect_success "'ipfs refs local' no longer shows file" '
  EMPTY_DIR=QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn &&
  HASH_MH=`cid-fmt -b base32 "%M" "$HASH"` &&
  HARDCODED_HASH_MH=`cid-fmt -b base32 "%M" "QmYCvbfNbCwFR45HiNP45rwJgvatpiW38D961L5qAhUM5Y"` &&
  EMPTY_DIR_MH=`cid-fmt -b base32 "%M" "$EMPTY_DIR"` &&
  HASH_WELCOME_DOCS_MH=`cid-fmt -b base32 "%M" "$HASH_WELCOME_DOCS"` &&
  ipfs refs local | cid-fmt -b base32 --filter "%M" >actual8 &&
  grep "$HARDCODED_HASH_MH" actual8 &&
  grep "$EMPTY_DIR_MH" actual8 &&
  grep "$HASH_WELCOME_DOCS_MH" actual8 &&
  test_must_fail grep "$HASH_MH" actual8
'

test_expect_success "adding multiblock random file succeeds" '
  random 1000000 >multiblock &&
  MBLOCKHASH=`ipfs add -q multiblock`
'

test_expect_success "'ipfs pin ls --type=indirect' is correct" '
  ipfs refs "$MBLOCKHASH" >refsout &&
  ipfs refs -r "$HASH_WELCOME_DOCS" >>refsout &&
  sed -i"~" "s/\(.*\)/\1 indirect/g" refsout &&
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
  ipfs pin add -r=false "$DIRECTPIN" >actual10 &&
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
  echo "$EMPTY_DIR" >>rp_expected &&
  sed -i"~" "s/\(.*\)/\1 recursive/g" rp_expected &&
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

test_expect_success "'ipfs refs --unique' is correct" '
  mkdir -p uniques &&
  echo "content1" > uniques/file1 &&
  echo "content1" > uniques/file2 &&
  ROOT=$(ipfs add -r -Q uniques) &&
  ipfs refs --unique $ROOT >expected &&
  ipfs add -q uniques/file1 >unique_hash &&
  test_cmp expected unique_hash
'

test_expect_success "'ipfs refs --unique --recursive' is correct" '
  mkdir -p a/b/c &&
  echo "c1" > a/f1 &&
  echo "c1" > a/b/f1 &&
  echo "c1" > a/b/c/f1 &&
  echo "c2" > a/b/c/f2 &&
  ROOT=$(ipfs add -r -Q a) &&
  ipfs refs --unique --recursive $ROOT >refs_output &&
  wc -l refs_output | sed "s/^ *//g" >line_count &&
  echo "4 refs_output" >expected &&
  test_cmp expected line_count || test_fsh cat refs_output
'

test_expect_success "'ipfs refs --recursive (bigger)'" '
  mkdir -p b/c/d/e &&
  echo "content1" >b/f &&
  echo "content1" >b/c/f1 &&
  echo "content1" >b/c/d/f2 &&
  echo "content2" >b/c/f2 &&
  echo "content2" >b/c/d/f1 &&
  echo "content2" >b/c/d/e/f &&
  cp -r b b2 && mv b2 b/b2 &&
  cp -r b b3 && mv b3 b/b3 &&
  cp -r b b4 && mv b4 b/b4 &&
  hash=$(ipfs add -r -Q b) &&
  ipfs refs -r "$hash" >refs_output &&
  wc -l refs_output | sed "s/^ *//g" >actual &&
  echo "79 refs_output" >expected &&
  test_cmp expected actual || test_fsh cat refs_output
'

test_expect_success "'ipfs refs --unique --recursive (bigger)'" '
  ipfs refs -r "$hash" >refs_output &&
  sort refs_output | uniq >expected &&
  ipfs refs -r -u "$hash" >actual &&
  test_sort_cmp expected actual || test_fsh cat refs_output
'

get_field_num() {
  field=$1
  file=$2
  num=$(grep "$field" "$file" | awk '{ print $2 }')
  echo $num
}

test_expect_success "'ipfs repo stat' succeeds" '
  ipfs repo stat > repo-stats
'

test_expect_success "repo stats came out correct" '
  grep "RepoPath" repo-stats &&
  grep "RepoSize" repo-stats &&
  grep "NumObjects" repo-stats &&
  grep "Version" repo-stats &&
  grep "StorageMax" repo-stats
'

test_expect_success "'ipfs repo stat --human' succeeds" '
  ipfs repo stat --human > repo-stats-human
'

test_expect_success "repo stats --human came out correct" '
  grep "RepoPath" repo-stats-human &&
  grep -E "RepoSize:\s*([0-9]*[.])?[0-9]+\s+?(B|kB|MB|GB|TB|PB|EB)" repo-stats-human &&
  grep "NumObjects" repo-stats-human &&
  grep "Version" repo-stats-human &&
  grep -E "StorageMax:\s*([0-9]*[.])?[0-9]+\s+?(B|kB|MB|GB|TB|PB|EB)" repo-stats-human ||
  test_fsh cat repo-stats-human
'

test_expect_success "'ipfs repo stat' after adding a file" '
  ipfs add repo-stats &&
  ipfs repo stat > repo-stats-2
'

test_expect_success "repo stats are updated correctly" '
  test $(get_field_num "RepoSize" repo-stats-2) -ge $(get_field_num "RepoSize" repo-stats)
'

test_expect_success "'ipfs repo stat --size-only' succeeds" '
  ipfs repo stat --size-only > repo-stats-size-only
'

test_expect_success "repo stats came out correct for --size-only" '
  grep "RepoSize" repo-stats-size-only &&
  grep "StorageMax" repo-stats-size-only &&
  grep -v "RepoPath" repo-stats-size-only &&
  grep -v "NumObjects" repo-stats-size-only &&
  grep -v "Version" repo-stats-size-only
'

test_expect_success "'ipfs repo version' succeeds" '
  ipfs repo version > repo-version
'

test_expect_success "repo version came out correct" '
  egrep "^ipfs repo version fs-repo@[0-9]+" repo-version >/dev/null
'

test_expect_success "'ipfs repo version -q' succeeds" '
  ipfs repo version -q > repo-version-q
'
test_expect_success "repo version came out correct" '
  egrep "^fs-repo@[0-9]+" repo-version-q >/dev/null
'

test_kill_ipfs_daemon

test_expect_success "remove Datastore.StorageMax from config" '
  ipfs config Datastore.StorageMax ""
'
test_expect_success "'ipfs repo stat' still succeeds" '
  ipfs repo stat > repo-stats
'

test_done
