#!/usr/bin/env bash

test_description="Test resolve command"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "resolve: prepare files" '
  mkdir -p a/b &&
  echo "a/b/c" >a/b/c &&
  a_hash=$(ipfs add -q -r a | tail -n1) &&
  b_hash=$(ipfs add -q -r a/b | tail -n1) &&
  c_hash=$(ipfs add -q -r a/b/c | tail -n1)
  a_hash_b32=$(cid-fmt -v 1 -b b %s $a_hash)
  b_hash_b32=$(cid-fmt -v 1 -b b %s $b_hash)
  c_hash_b32=$(cid-fmt -v 1 -b b %s $c_hash)
'

test_resolve_setup_name() {
  ref=$1

  test_expect_success "resolve: prepare name" '
    id_hash=$(ipfs id -f="<id>") &&
    ipfs name publish --allow-offline "$ref" &&
    printf "$ref\n" >expected_nameval &&
    ipfs name resolve >actual_nameval &&
    test_cmp expected_nameval actual_nameval
  '
}

test_resolve_setup_name_fail() {
  ref=$1

  test_expect_failure "resolve: prepare name" '
    id_hash=$(ipfs id -f="<id>") &&
    ipfs name publish --allow-offline "$ref" &&
    printf "$ref" >expected_nameval &&
    ipfs name resolve >actual_nameval &&
    test_cmp expected_nameval actual_nameval
  '
}

test_resolve() {
  src=$1
  dst=$2
  extra=$3

  test_expect_success "resolve succeeds: $src" '
    ipfs resolve $extra -r "$src" >actual
  '

  test_expect_success "resolved correctly: $src -> $dst" '
    printf "$dst\n" >expected &&
    test_cmp expected actual
  '
}

test_resolve_cmd() {
  test_resolve "/ipfs/$a_hash" "/ipfs/$a_hash"
  test_resolve "/ipfs/$a_hash/b" "/ipfs/$b_hash"
  test_resolve "/ipfs/$a_hash/b/c" "/ipfs/$c_hash"
  test_resolve "/ipfs/$b_hash/c" "/ipfs/$c_hash"

  test_resolve_setup_name "/ipfs/$a_hash"
  test_resolve "/ipns/$id_hash" "/ipfs/$a_hash"
  test_resolve "/ipns/$id_hash/b" "/ipfs/$b_hash"
  test_resolve "/ipns/$id_hash/b/c" "/ipfs/$c_hash"

  test_resolve_setup_name "/ipfs/$b_hash"
  test_resolve "/ipns/$id_hash" "/ipfs/$b_hash"
  test_resolve "/ipns/$id_hash/c" "/ipfs/$c_hash"

  test_resolve_setup_name "/ipfs/$c_hash"
  test_resolve "/ipns/$id_hash" "/ipfs/$c_hash"
}

test_resolve_cmd_b32() {
  # no flags needed, base should be preserved

  test_resolve "/ipfs/$a_hash_b32" "/ipfs/$a_hash_b32"
  test_resolve "/ipfs/$a_hash_b32/b" "/ipfs/$b_hash_b32"
  test_resolve "/ipfs/$a_hash_b32/b/c" "/ipfs/$c_hash_b32"
  test_resolve "/ipfs/$b_hash_b32/c" "/ipfs/$c_hash_b32"

  # flags needed passed in path does not contain cid to derive base
 
  test_resolve_setup_name "/ipfs/$a_hash_b32"
  test_resolve "/ipns/$id_hash" "/ipfs/$a_hash_b32" --cid-base=base32
  test_resolve "/ipns/$id_hash/b" "/ipfs/$b_hash_b32" --cid-base=base32
  test_resolve "/ipns/$id_hash/b/c" "/ipfs/$c_hash_b32" --cid-base=base32

  test_resolve_setup_name "/ipfs/$b_hash_b32" --cid-base=base32
  test_resolve "/ipns/$id_hash" "/ipfs/$b_hash_b32" --cid-base=base32
  test_resolve "/ipns/$id_hash/c" "/ipfs/$c_hash_b32" --cid-base=base32

  test_resolve_setup_name "/ipfs/$c_hash_b32"
  test_resolve "/ipns/$id_hash" "/ipfs/$c_hash_b32" --cid-base=base32
}


#todo remove this once the online resolve is fixed
test_resolve_fail() {
  src=$1
  dst=$2

  test_expect_failure "resolve succeeds: $src" '
    ipfs resolve "$src" >actual
  '

  test_expect_failure "resolved correctly: $src -> $dst" '
    printf "$dst" >expected &&
    test_cmp expected actual
  '
}

test_resolve_cmd_fail() {
  test_resolve "/ipfs/$a_hash" "/ipfs/$a_hash"
  test_resolve "/ipfs/$a_hash/b" "/ipfs/$b_hash"
  test_resolve "/ipfs/$a_hash/b/c" "/ipfs/$c_hash"
  test_resolve "/ipfs/$b_hash/c" "/ipfs/$c_hash"

  test_resolve_setup_name_fail "/ipfs/$a_hash"
  test_resolve_fail "/ipns/$id_hash" "/ipfs/$a_hash"
  test_resolve_fail "/ipns/$id_hash/b" "/ipfs/$b_hash"
  test_resolve_fail "/ipns/$id_hash/b/c" "/ipfs/$c_hash"

  test_resolve_setup_name_fail "/ipfs/$b_hash"
  test_resolve_fail "/ipns/$id_hash" "/ipfs/$b_hash"
  test_resolve_fail "/ipns/$id_hash/c" "/ipfs/$c_hash"

  test_resolve_setup_name_fail "/ipfs/$c_hash"
  test_resolve_fail "/ipns/$id_hash" "/ipfs/$c_hash"
}

# should work offline
test_resolve_cmd
test_resolve_cmd_b32

# should work online
test_launch_ipfs_daemon
test_resolve_cmd_fail
test_kill_ipfs_daemon

test_done
