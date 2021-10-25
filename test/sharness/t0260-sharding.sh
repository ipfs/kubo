#!/usr/bin/env bash
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test directory sharding"

. lib/test-lib.sh

# We shard based on size with a threshold of 256 KiB (see `HAMTShardingSize`
#  in core/node/groups.go) above which directories are sharded.
# The directory size is estimated as the size of each link (roughly entry name,
#  here of always 10 chars, and CID byte length, normally 34 bytes). So we need
#  256 KiB / (34 + 10) ~ 6000 entries in the directory to trigger sharding.
# We create then two directories: one above the threshold (big_dir) and one
# below (small_dir), and hard-code the CIDs of their sharded and unsharded
# codifications as IPFS directories.
test_expect_success "set up test data" '
  mkdir big_dir
  for i in `seq 5960` # Just above the number of entries that trigger sharding for 256KiB
  do
    echo $i > big_dir/`printf "file%06d" $i` # fixed length of 10 chars
  done

  mkdir small_dir
  for i in `seq 100`
  do
    echo $i > small_dir/`printf "file%06d" $i`
  done
'
# CID of big_dir/ which will be sharded.
SHARDED="QmWfjnRWRvdvYezQWnfbvrvY7JjrpevsE9cato1x76UqGr"
# CID of big_dir/ once we remove half its entries and trigger a switch
# back to a basic directory (un-sharding).
UNSHAREDED="QmbVxi5zDdzytrjdufUejM92JsWj8wGVmukk6tiPce3p1m"
# CID of small_dir/ which will *not* be sharded.
NOT_SHARDED="QmdBXmm4HRpUhyzzctbFvi2tLai3XFL1YjmE1qfpJe61NX"

test_add_dir_with_sharding_enabled() {
  exphash="$1"
  input_dir="$2"
  test_expect_success "ipfs add directory (with sharding enabled) succeeds" '
    ipfs add -r -Q $input_dir > sharddir_out &&
    echo "$exphash" > sharddir_exp &&
    test_cmp sharddir_exp sharddir_out
  '
  test_expect_success "ipfs add directory (with sharding enabled) succeeds matches expected hash" '
    ipfs get -o output_dir "$exphash" &&
    test_cmp $input_dir output_dir
    rm output_dir -r
  '
}

test_init_ipfs

test_add_dir_with_sharding_enabled "$SHARDED" big_dir
test_add_dir_with_sharding_enabled "$NOT_SHARDED" small_dir

test_launch_ipfs_daemon

test_add_dir_with_sharding_enabled "$SHARDED" big_dir
test_add_dir_with_sharding_enabled "$NOT_SHARDED" small_dir

test_kill_ipfs_daemon

test_expect_success "remove a few entries from big_dir/ to trigger unsharding" '
  ipfs files cp /ipfs/$SHARDED /big_dir &&
  for i in `seq 5`
  do
    ipfs files rm /big_dir/`printf "file%06d" $i`
  done &&
  ipfs files stat --hash /big_dir > unshard_dir_hash &&
  echo "$UNSHAREDED" > unshard_exp &&
  test_cmp unshard_exp unshard_dir_hash
'

test_expect_success "ipfs cat error output the same" '
  test_expect_code 1 ipfs cat "$SHARDED" 2> sharded_err &&
  test_expect_code 1 ipfs cat "$NOT_SHARDED" 2> unsharded_err &&
  test_cmp sharded_err unsharded_err
'

test_expect_success "'ipfs ls --resolve-type=false --size=false' admits missing block" '
  ipfs ls "$SHARDED" | head -1 > first_file &&
  ipfs ls --size=false "$SHARDED" | sort > sharded_out_nosize &&
  read -r HASH _ NAME <first_file &&
  ipfs pin rm "$SHARDED" "$NOT_SHARDED" && # To allow us to remove the block
  ipfs block rm "$HASH" &&
  test_expect_code 1 ipfs cat "$SHARDED/$NAME" &&
  test_expect_code 1 ipfs ls "$SHARDED" &&
  ipfs ls --resolve-type=false --size=false "$SHARDED" | sort > missing_out &&
  test_cmp sharded_out_nosize missing_out
'

test_launch_ipfs_daemon

test_expect_success "gateway can resolve sharded dirs" '
  echo 100 > expected &&
  curl -sfo actual "http://127.0.0.1:$GWAY_PORT/ipfs/$SHARDED/file000100" &&
  test_cmp expected actual
'

test_expect_success "'ipfs resolve' can resolve sharded dirs" '
  echo /ipfs/QmZ3RfWk1u5LEGYLHA633B5TNJy3Du27K6Fny9wcxpowGS > expected &&
  ipfs resolve "/ipfs/$SHARDED/file000100" > actual &&
  test_cmp expected actual
'

test_kill_ipfs_daemon

test_add_dir_with_sharding_enabled_v1() {
  exphash="$1"
  input_dir="$2"
  test_expect_success "ipfs add (CIDv1) on very large directory succeeds" '
    ipfs add -r -Q --cid-version=1 "$input_dir" > sharddir_out &&
    echo "$exphash" > sharddir_exp &&
    test_cmp sharddir_exp sharddir_out
  '

  test_expect_success "can access a path under the dir" '
    ipfs cat "$exphash/file003020" > file3020_out &&
    test_cmp "$input_dir/file003020" file3020_out
  '
}

# this hash implies the directory is CIDv1 and leaf entries are CIDv1 and raw
SHARDEDV1="bafybeiglv7wdwjqaqxjs4nk3uo4vvldhmmod5vkinvgwihyrv6bdzuzbbm"
test_add_dir_with_sharding_enabled_v1 "$SHARDEDV1" big_dir

test_launch_ipfs_daemon

test_add_dir_with_sharding_enabled_v1 "$SHARDEDV1" big_dir

test_kill_ipfs_daemon

test_list_incomplete_dir() {
  test_expect_success "ipfs add (CIDv1) on very large directory with sha3 succeeds" '
    ipfs add -r -Q --cid-version=1 --hash=sha3-256 --pin=false testdata > sharddir_out &&
    largeSHA3dir=$(cat sharddir_out)
  '

  test_expect_success "delete intermediate node from DAG" '
    ipfs block rm "/ipld/$largeSHA3dir/Links/0/Hash"
  '

  test_expect_success "can list part of the directory" '
    ipfs ls "$largeSHA3dir" 2> ls_err_out
    echo "Error: merkledag: not found" > exp_err_out &&
    cat ls_err_out &&
    test_cmp exp_err_out ls_err_out
  '
}

test_list_incomplete_dir

test_done
