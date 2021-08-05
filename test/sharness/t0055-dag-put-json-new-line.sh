#!/usr/bin/env bash

test_description='Test retrieval of JSON put as CBOR does not end with new-line'

. lib/test-lib.sh

test_init_ipfs

test_expect_success 'create test JSON files' '
  WANT_JSON="{\"data\":1234}"
  WANT_HASH="bafyreidcqah6v3py3ujdc3ris22rjlfntaw7ajrus2f2477kpnizulaoea"
  printf "${WANT_JSON}\n" > with_newline.json &&
  printf "${WANT_JSON}" > without_newline.json
'

test_expect_success 'puts as CBOR work' '
  GOT_HASH_WITHOUT_NEWLINE="$(cat without_newline.json | ipfs dag put -f cbor)"
  GOT_HASH_WITH_NEWLINE="$(cat with_newline.json | ipfs dag put -f cbor)"
'

test_expect_success 'put hashes with or without newline are equal' '
  test "${GOT_HASH_WITH_NEWLINE}" = "${GOT_HASH_WITHOUT_NEWLINE}"
'

test_expect_success 'hashes are of expected value' '
  test "${WANT_HASH}" = "${GOT_HASH_WITH_NEWLINE}"
  test "${WANT_HASH}" = "${GOT_HASH_WITHOUT_NEWLINE}"
'

# Retrieval must not contain a new-line regardless of input JSON, because
# objects are put using the stable CBOR format.
# despite this, dag retrieval returns JSON with new-line.
# Expect failure until fixed, as per:
# - https://github.com/ipfs/go-ipfs/issues/3503#issuecomment-877295280
test_expect_failure "retrieval by hash does not have new line" '
  ipfs dag get "${WANT_HASH}" > got.json
  test_cmp without_newline.json got.json
'

test_done
