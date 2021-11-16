#!/usr/bin/env bash
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test directory sharding"

. lib/test-lib.sh

# We shard based on size with a threshold of 256 KiB (see config file docs)
# above which directories are sharded.
#
# The directory size is estimated as the size of each link. Links are roughly
# the entry name + the CID byte length (e.g. 34 bytes for a CIDv0). So for
# entries of length 10 we need 256 KiB / (34 + 10) ~ 6000 entries in the
# directory to trigger sharding.
test_expect_success "set up test data" '
  mkdir testdata
  for i in `seq 2000`
  do
    echo $i > testdata/file$i
  done

  mkdir big_dir
  for i in `seq 5960` # Just above the number of entries that trigger sharding for 256KiB
  do
    echo $i > big_dir/`printf "file%06d" $i` # fixed length of 10 chars
  done
'

test_add_dir() {
  exphash="$1"
  test_expect_success "ipfs add on directory succeeds" '
    ipfs add -r -Q testdata > sharddir_out &&
    echo "$exphash" > sharddir_exp &&
    test_cmp sharddir_exp sharddir_out
  '
  test_expect_success "ipfs get on directory succeeds" '
    ipfs get -o testdata-out "$exphash" &&
    test_cmp testdata testdata-out
  '
}

test_init_ipfs

UNSHARDED="QmavrTrQG4VhoJmantURAYuw3bowq3E2WcvP36NRQDAC1N"

test_expect_success "force sharding off" '
ipfs config --json Internal.UnixFSShardingSizeThreshold "\"1G\""
'

test_add_dir "$UNSHARDED"

test_launch_ipfs_daemon

test_add_dir "$UNSHARDED"

test_kill_ipfs_daemon

test_expect_success "force sharding on" '
  ipfs config --json Internal.UnixFSShardingSizeThreshold "\"1B\""
'

SHARDED="QmSCJD1KYLhVVHqBK3YyXuoEqHt7vggyJhzoFYbT8v1XYL"
test_add_dir "$SHARDED"

test_launch_ipfs_daemon

test_add_dir "$SHARDED"

test_kill_ipfs_daemon

test_expect_success "sharded and unsharded output look the same" '
  ipfs ls "$SHARDED" | sort > sharded_out &&
  ipfs ls "$UNSHARDED" | sort > unsharded_out &&
  test_cmp sharded_out unsharded_out
'

test_expect_success "ipfs cat error output the same" '
  test_expect_code 1 ipfs cat "$SHARDED" 2> sharded_err &&
  test_expect_code 1 ipfs cat "$UNSHARDED" 2> unsharded_err &&
  test_cmp sharded_err unsharded_err
'

test_expect_success "'ipfs ls --resolve-type=false --size=false' admits missing block" '
  ipfs ls "$SHARDED" | head -1 > first_file &&
  ipfs ls --size=false "$SHARDED" | sort > sharded_out_nosize &&
  read -r HASH _ NAME <first_file &&
  ipfs pin rm "$SHARDED" "$UNSHARDED" && # To allow us to remove the block
  ipfs block rm "$HASH" &&
  test_expect_code 1 ipfs cat "$SHARDED/$NAME" &&
  test_expect_code 1 ipfs ls "$SHARDED" &&
  ipfs ls --resolve-type=false --size=false "$SHARDED" | sort > missing_out &&
  test_cmp sharded_out_nosize missing_out
'

test_launch_ipfs_daemon

test_expect_success "gateway can resolve sharded dirs" '
  echo 100 > expected &&
  curl -sfo actual "http://127.0.0.1:$GWAY_PORT/ipfs/$SHARDED/file100" &&
  test_cmp expected actual
'

test_expect_success "'ipfs resolve' can resolve sharded dirs" '
  echo /ipfs/QmZ3RfWk1u5LEGYLHA633B5TNJy3Du27K6Fny9wcxpowGS > expected &&
  ipfs resolve "/ipfs/$SHARDED/file100" > actual &&
  test_cmp expected actual
'

test_kill_ipfs_daemon

test_add_dir_v1() {
  exphash="$1"
  test_expect_success "ipfs add (CIDv1) on directory succeeds" '
    ipfs add -r -Q --cid-version=1 testdata > sharddir_out &&
    echo "$exphash" > sharddir_exp &&
    test_cmp sharddir_exp sharddir_out
  '

  test_expect_success "can access a path under the dir" '
    ipfs cat "$exphash/file20" > file20_out &&
    test_cmp testdata/file20 file20_out
  '
}

# this hash implies the directory is CIDv1 and leaf entries are CIDv1 and raw
SHARDEDV1="bafybeibiemewfzzdyhq2l74wrd6qj2oz42usjlktgnlqv4yfawgouaqn4u"
test_add_dir_v1 "$SHARDEDV1"

test_launch_ipfs_daemon

test_add_dir_v1 "$SHARDEDV1"

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
    echo "Error: failed to fetch all nodes" > exp_err_out &&
    cat ls_err_out &&
    test_cmp exp_err_out ls_err_out
  '
}

test_list_incomplete_dir

# Test automatic sharding and unsharding

# TODO: This does not need to report an error https://github.com/ipfs/go-ipfs/issues/8088
test_expect_failure "reset automatic sharding" '
  ipfs config --json Internal.UnixFSShardingSizeThreshold null
'

LARGE_SHARDED="QmWfjnRWRvdvYezQWnfbvrvY7JjrpevsE9cato1x76UqGr"
LARGE_MINUS_5_UNSHAREDED="QmbVxi5zDdzytrjdufUejM92JsWj8wGVmukk6tiPce3p1m"

test_add_large_sharded_dir() {
  exphash="$1"
  test_expect_success "ipfs add on directory succeeds" '
    ipfs add -r -Q big_dir > shardbigdir_out &&
    echo "$exphash" > shardbigdir_exp &&
    test_cmp shardbigdir_exp shardbigdir_out
  '

  test_expect_success "can access a path under the dir" '
    ipfs cat "$exphash/file000030" > file30_out &&
    test_cmp big_dir/file000030 file30_out
  '
}

test_add_large_sharded_dir "$LARGE_SHARDED"

test_expect_success "remove a few entries from big_dir/ to trigger unsharding" '
  ipfs files cp /ipfs/"$LARGE_SHARDED" /big_dir &&
  for i in `seq 5`
  do
    ipfs files rm /big_dir/`printf "file%06d" $i`
  done &&
  ipfs files stat --hash /big_dir > unshard_dir_hash &&
  echo "$LARGE_MINUS_5_UNSHAREDED" > unshard_exp &&
  test_cmp unshard_exp unshard_dir_hash
'

test_expect_success "add a few entries to big_dir/ to retrigger sharding" '
  for i in `seq 5`
  do
    ipfs files cp /ipfs/"$LARGE_SHARDED"/`printf "file%06d" $i` /big_dir/`printf "file%06d" $i`
  done &&
  ipfs files stat --hash /big_dir > shard_dir_hash &&
  echo "$LARGE_SHARDED" > shard_exp &&
  test_cmp shard_exp shard_dir_hash
'

test_done
