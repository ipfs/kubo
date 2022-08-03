#!/usr/bin/env bash
#
# Copyright (c) 2020 Claudia Richoux
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test blake3 mhash support"

. lib/test-lib.sh

test_init_ipfs

# the blake3 hash of "foo\n" in UTF8 (which is what comes out of echo when you pipe into `ipfs`) starts with "49dc870df1de7fd60794cebce449f5ccdae575affaa67a24b62acb03e039db92"
# without the newline it's "04e0bb39f30b1a3feb89f536c93be15055482df748674b00d26e5a75777702e9". so if you start seeing these values that's your problem
BLAKE3RAWCID32BYTE="bafkr4icj3sdq34o6p7lapfgoxtset5om3lsxll72uz5cjnrkzmb6aoo3si"
BLAKE3RAWCID64BYTE="bafkr4qcj3sdq34o6p7lapfgoxtset5om3lsxll72uz5cjnrkzmb6aoo3sknmbprpe27pbfrb67tonydgfot5ixuq4skiva76ppbgpjlzc4ua4"
BLAKE3RAWCID128BYTE="bafkr5aabjhoiodpr3z75mb4uz26oispvztnok5np7kthujfwflfqhybz3ojjvqf6f4tl54eweh36nzxamyv2pvc6sdsjjcud7z54ez5fpelsqdtax2k3rvuq3wdl5a4blxv3gvsroa3nakfzzknamhu2apf3vvytyiobabrn2bfnfajq66ikjy5lewsp5jyddsg5l7u3emr2ancimryay"

### block tests, including for various sizes of hash ###

test_expect_success "putting a block with an mhash blake3 succeeds (default 32 bytes)" '
  HASH=$(echo "foo" | ipfs block put --mhtype=blake3 --cid-codec=raw | tee actual_out) &&
  test $BLAKE3RAWCID32BYTE = "$HASH"
'

test_expect_success "block get output looks right" '
  ipfs block get $BLAKE3RAWCID32BYTE > blk_get_out &&
  echo "foo" > blk_get_exp &&
  test_cmp blk_get_exp blk_get_out
'

test_expect_success "putting a block with an mhash blake3 succeeds: 64 bytes" '
  HASH=$(echo "foo" | ipfs block put --mhtype=blake3 --mhlen=64 --cid-codec=raw | tee actual_out) &&
  test $BLAKE3RAWCID64BYTE = "$HASH"
'

test_expect_success "64B block get output looks right" '
  ipfs block get $BLAKE3RAWCID64BYTE > blk_get_out &&
  echo "foo" > blk_get_exp &&
  test_cmp blk_get_exp blk_get_out
'

test_expect_success "putting a block with an mhash blake3 succeeds: 128 bytes" '
  HASH=$(echo "foo" | ipfs block put --mhtype=blake3 --mhlen=128 --cid-codec=raw | tee actual_out) &&
  test $BLAKE3RAWCID128BYTE = "$HASH"
'

test_expect_success "32B block get output looks right" '
  ipfs block get $BLAKE3RAWCID128BYTE > blk_get_out &&
  echo "foo" > blk_get_exp &&
  test_cmp blk_get_exp blk_get_out
'

### dag tests ###

test_expect_success "dag put works with blake3" '
  HASH=$(echo "foo" | ipfs dag put --input-codec=raw --store-codec=raw --hash=blake3 | tee actual_out) &&
  test $BLAKE3RAWCID32BYTE = "$HASH"
'

test_expect_success "dag get output looks right" '
  ipfs dag get --output-codec=raw $BLAKE3RAWCID32BYTE > dag_get_out &&
  echo "foo" > dag_get_exp &&
  test_cmp dag_get_exp dag_get_out
'

### add and cat tests ###

test_expect_success "adding a file with just foo in it to ipfs" '
  echo "foo" > afile &&
  HASH=$(ipfs add -q --hash=blake3 --raw-leaves afile | tee actual_out) &&
  test $BLAKE3RAWCID32BYTE = "$HASH"
'

test_expect_success "catting it" '
  ipfs cat $BLAKE3RAWCID32BYTE > cat_out &&
  echo "foo" > cat_exp &&
  test_cmp cat_exp cat_out
'

test_done
