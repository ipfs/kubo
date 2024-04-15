#!/usr/bin/env bash
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test block command"

. lib/test-lib.sh

test_init_ipfs

HASH="bafkreibmlvvgdyihetgocpof6xk64kjjzdeq2e4c7hqs3krdheosk4tgj4"
HASHB="bafkreihfsphazrk2ilejpekyltjeh5k4yvwgjuwg26ueafohqioeo3sdca"

HASHV0="QmRKqGMAM6EZngbpjSqrvYzq5Qd8b1bSWymjSUY9zQSNDk"
HASHBV0="QmdnpnsaEj69isdw5sNzp3h3HkaDz7xKq7BmvFFBzNr5e7"

# "block put tests"
#

test_expect_success "'ipfs block put' succeeds" '
  echo "Hello Mars!" >expected_in &&
  ipfs block put <expected_in | tee actual_out
'

test_expect_success "'ipfs block put' output looks good" '
  echo "$HASH" >expected_out &&
  test_cmp expected_out actual_out
'

test_expect_success "'ipfs block put' with 2 files succeeds" '
  echo "Hello Mars!" > a &&
  echo "Hello Venus!" > b &&
  ipfs block put a b | tee actual_out
'

test_expect_success "'ipfs block put' output looks good" '
  echo "$HASH" >expected_out &&
  echo "$HASHB" >>expected_out &&
  test_cmp expected_out actual_out
'

test_expect_success "can set cid codec on block put" '
  CODEC_HASH=$(ipfs block put --cid-codec=dag-pb ../t0050-block-data/testPut.pb)
'

test_expect_success "block get output looks right" '
  ipfs block get $CODEC_HASH > pb_block_out &&
  test_cmp pb_block_out ../t0050-block-data/testPut.pb
'

#
# "block get" tests
#

test_expect_success "'ipfs block get' succeeds" '
  ipfs block get $HASH >actual_in
'

test_expect_success "'ipfs block get' output looks good" '
  test_cmp expected_in actual_in
'

#
# "block stat" tests
#

test_expect_success "'ipfs block stat' succeeds" '
  ipfs block stat $HASH >actual_stat
'

test_expect_success "'ipfs block stat' output looks good" '
  echo "Key: $HASH" >expected_stat &&
  echo "Size: 12" >>expected_stat &&
  test_cmp expected_stat actual_stat
'

#
# "block rm" tests
#

test_expect_success "'ipfs block rm' succeeds" '
  ipfs block rm $HASH >actual_rm
'

test_expect_success "'ipfs block rm' output looks good" '
  echo "removed $HASH" > expected_rm &&
  test_cmp expected_rm actual_rm
'

test_expect_success "'ipfs block rm' block actually removed" '
  test_must_fail ipfs block stat $HASH
'

RANDOMHASH=QmRKqGMAM6EbngbZjSqrvYzq5Qd8b1bSWymjSUY9zQSNDq
DIRHASH=QmdWmVmM6W2abTgkEfpbtA1CJyTWS2rhuUB9uP1xV8Uwtf
FILE1HASH=Qmae3RedM7SNkWGsdzYzsr6svmsFdsva4WoTvYYsWhUSVz
FILE2HASH=QmUtkGLvPf63NwVzLPKPUYgwhn8ZYPWF6vKWN3fZ2amfJF
FILE3HASH=Qmesmmf1EEG1orJb6XdK6DabxexsseJnCfw8pqWgonbkoj
TESTHASH=QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH

test_expect_success "add and pin directory" '
  echo "test" | ipfs add --pin=false &&
  mkdir adir &&
  echo "file1" > adir/file1 &&
  echo "file2" > adir/file2 &&
  echo "file3" > adir/file3 &&
  ipfs add -r adir
  ipfs pin add -r $DIRHASH
'

test_expect_success "can't remove pinned block" '
  test_must_fail ipfs block rm $DIRHASH 2> block_rm_err
'

test_expect_success "can't remove pinned block: output looks good" '
  grep -q "$DIRHASH: pinned: recursive" block_rm_err
'

test_expect_success "can't remove indirectly pinned block" '
  test_must_fail ipfs block rm $FILE1HASH 2> block_rm_err
'

test_expect_success "can't remove indirectly pinned block: output looks good" '
  grep -q "$FILE1HASH: pinned via $DIRHASH" block_rm_err
'

test_expect_success "remove pin" '
  ipfs pin rm -r $DIRHASH
'

test_expect_success "multi-block 'ipfs block rm' succeeds" '
  ipfs block rm $FILE1HASH $FILE2HASH $FILE3HASH > actual_rm
'

test_expect_success "multi-block 'ipfs block rm' output looks good" '
  grep -F -q "removed $FILE1HASH" actual_rm &&
  grep -F -q "removed $FILE2HASH" actual_rm &&
  grep -F -q "removed $FILE3HASH" actual_rm
'

test_expect_success "multi-block 'ipfs block rm <invalid> <valid> <invalid>'" '
  test_must_fail ipfs block rm $RANDOMHASH $TESTHASH $RANDOMHASH &> actual_mixed_rm
'

test_expect_success "multi-block 'ipfs block rm <invalid> <valid> <invalid>' output looks good" '
  echo "cannot remove $RANDOMHASH: ipld: could not find $RANDOMHASH" >> expect_mixed_rm &&
  echo "removed $TESTHASH" >> expect_mixed_rm &&
  echo "cannot remove $RANDOMHASH: ipld: could not find $RANDOMHASH" >> expect_mixed_rm &&
  echo "Error: some blocks not removed" >> expect_mixed_rm
  test_cmp actual_mixed_rm expect_mixed_rm
'

test_expect_success "'add some blocks' succeeds" '
  echo "Hello Mars!" | ipfs block put &&
  echo "Hello Venus!" | ipfs block put
'

test_expect_success "add and pin directory" '
  ipfs add -r adir
  ipfs pin add -r $DIRHASH
'

HASH=QmRKqGMAM6EZngbpjSqrvYzq5Qd8b1bSWymjSUY9zQSNDk
HASH2=QmdnpnsaEj69isdw5sNzp3h3HkaDz7xKq7BmvFFBzNr5e7

test_expect_success "multi-block 'ipfs block rm' mixed" '
  test_must_fail ipfs block rm $FILE1HASH $DIRHASH $HASH $FILE3HASH $RANDOMHASH $HASH2 2> block_rm_err
'

test_expect_success "pinned block not removed" '
  ipfs block stat $FILE1HASH &&
  ipfs block stat $FILE3HASH
'

test_expect_success "non-pinned blocks removed" '
  test_must_fail ipfs block stat $HASH &&
  test_must_fail ipfs block stat $HASH2
'

test_expect_success "error reported on removing non-existent block" '
  grep -q "cannot remove $RANDOMHASH" block_rm_err
'

test_expect_success "'add some blocks' succeeds" '
  echo "Hello Mars!" | ipfs block put &&
  echo "Hello Venus!" | ipfs block put
'

test_expect_success "multi-block 'ipfs block rm -f' with non existent blocks succeed" '
  ipfs block rm -f $HASH $RANDOMHASH $HASH2
'

test_expect_success "existent blocks removed" '
  test_must_fail ipfs block stat $HASH &&
  test_must_fail ipfs block stat $HASH2
'

test_expect_success "'add some blocks' succeeds" '
  echo "Hello Mars!" | ipfs block put &&
  echo "Hello Venus!" | ipfs block put
'

test_expect_success "multi-block 'ipfs block rm -q' produces no output" '
  ipfs block rm -q $HASH $HASH2 > block_rm_out &&
  test ! -s block_rm_out
'

# --format used 'protobuf' for 'dag-pb' which was invalid, but we keep
# for backward-compatibility
test_expect_success "can set deprecated --format=protobuf on block put" '
  HASH=$(ipfs block put --format=protobuf ../t0050-block-data/testPut.pb)
'

test_expect_success "created an object correctly!" '
  ipfs dag get $HASH > obj_out &&
  echo -n "{\"Data\":{\"/\":{\"bytes\":\"dGVzdCBqc29uIGZvciBzaGFybmVzcyB0ZXN0\"}},\"Links\":[]}" > obj_exp &&
  test_cmp obj_out obj_exp
'

test_expect_success "block get output looks right" '
  ipfs block get $HASH > pb_block_out &&
  test_cmp pb_block_out ../t0050-block-data/testPut.pb
'

test_expect_success "can set --cid-codec=dag-pb on block put" '
  HASH=$(ipfs block put --cid-codec=dag-pb ../t0050-block-data/testPut.pb)
'

test_expect_success "created an object correctly!" '
  ipfs dag get $HASH > obj_out &&
  echo -n "{\"Data\":{\"/\":{\"bytes\":\"dGVzdCBqc29uIGZvciBzaGFybmVzcyB0ZXN0\"}},\"Links\":[]}" > obj_exp &&
  test_cmp obj_out obj_exp
'

test_expect_success "block get output looks right" '
  ipfs block get $HASH > pb_block_out &&
  test_cmp pb_block_out ../t0050-block-data/testPut.pb
'

test_expect_success "can set multihash type and length on block put with --format=raw (deprecated)" '
  HASH=$(echo "foooo" | ipfs block put --format=raw --mhtype=sha3 --mhlen=20)
'

test_expect_success "output looks good" '
  test "bafkrifctrq4xazzixy2v4ezymjcvzpskqdwlxra" = "$HASH"
'

test_expect_success "can't use both legacy format and custom cid-codec at the same time" '
  test_expect_code 1 ipfs block put --format=dag-cbor --cid-codec=dag-json < ../t0050-block-data/testPut.pb 2> output &&
  test_should_contain "unable to use \"format\" (deprecated) and a custom \"cid-codec\" at the same time" output
'

test_expect_success "can read block with different hash" '
  ipfs block get $HASH > blk_get_out &&
  echo "foooo" > blk_get_exp &&
  test_cmp blk_get_exp blk_get_out
'
#
# Misc tests
#

test_expect_success "'ipfs block stat' with nothing from stdin doesn't crash" '
  test_expect_code 1 ipfs block stat < /dev/null 2> stat_out
'

# lol
test_expect_success "no panic in output" '
  test_expect_code 1 grep "panic" stat_out
'

test_expect_success "can set multihash type and length on block put without format or cid-codec" '
  HASH=$(echo "foooo" | ipfs block put --mhtype=sha3 --mhlen=20)
'

test_expect_success "output looks good" '
  test "bafkrifctrq4xazzixy2v4ezymjcvzpskqdwlxra" = "$HASH"
'

test_expect_success "can set multihash type and length on block put with cid-codec=dag-pb" '
  HASH=$(echo "foooo" | ipfs block put --mhtype=sha3 --mhlen=20 --cid-codec=dag-pb)
'

test_expect_success "output looks good" '
  test "bafybifctrq4xazzixy2v4ezymjcvzpskqdwlxra" = "$HASH"
'

test_expect_success "put with sha3 and cidv0 fails" '
  echo "foooo" | test_must_fail ipfs block put --mhtype=sha3 --mhlen=20 --format=v0
'

test_expect_success "'ipfs block put' check block size" '
    dd if=/dev/zero bs=2MB count=1 > 2-MB-file &&
    test_expect_code 1 ipfs block put 2-MB-file >block_put_out 2>&1
  '

  test_expect_success "ipfs block put output has the correct error" '
    grep "produced block is over 1MiB" block_put_out
  '

  test_expect_success "ipfs block put --allow-big-block=true works" '
    test_expect_code 0 ipfs block put 2-MB-file --allow-big-block=true &&
    rm 2-MB-file
  '

test_done
