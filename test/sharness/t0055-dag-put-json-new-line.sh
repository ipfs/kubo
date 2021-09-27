#!/usr/bin/env bash

test_description='Test retrieval of JSON put as CBOR does not end with new-line'

. lib/test-lib.sh

test_init_ipfs

test_expect_success 'create test JSON files' '
  WANT_JSON="{\"data\":1234}"
  WANT_HASH="bafyreidbm2zncsc3j25zn7lofgd4woeh6eygdy73thfosuni2rwr3bhcvu"
  printf "${WANT_JSON}\n" > with_newline.json &&
  printf "${WANT_JSON}" > without_newline.json
'

test_expect_success 'puts as CBOR work' '
  GOT_HASH_WITHOUT_NEWLINE="$(cat without_newline.json | ipfs dag put --store-codec dag-cbor)"
  GOT_HASH_WITH_NEWLINE="$(cat with_newline.json | ipfs dag put --store-codec dag-cbor)"
'

test_expect_success 'put hashes with or without newline are equal' '
  test "${GOT_HASH_WITH_NEWLINE}" = "${GOT_HASH_WITHOUT_NEWLINE}"
'

test_expect_success 'hashes are of expected value' '
  test "${WANT_HASH}" = "${GOT_HASH_WITH_NEWLINE}"
  test "${WANT_HASH}" = "${GOT_HASH_WITHOUT_NEWLINE}"
'

test_expect_success "retrieval by hash does not have new line" '
  ipfs dag get "${WANT_HASH}" > got.json
  test_cmp without_newline.json got.json
'

test_done
