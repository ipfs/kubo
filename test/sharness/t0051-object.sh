#!/usr/bin/env bash
#
# Copyright (c) 2015 Henry Bubert
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test object command"

. lib/test-lib.sh

test_init_ipfs

test_patch_create_path() {
  root=$1
  name=$2
  target=$3

  test_expect_success "object patch --create works" '
    PCOUT=$(ipfs object patch $root add-link --create $name $target)
  '

  test_expect_success "output looks good" '
    ipfs cat "$PCOUT/$name" >tpcp_out &&
    ipfs cat "$target" >tpcp_exp &&
    test_cmp tpcp_exp tpcp_out
  '
}

test_object_cmd() {
  EMPTY_DIR=$(echo '{"Links":[]}' | ipfs dag put --store-codec dag-pb)
  EMPTY_UNIXFS_DIR=$(echo '{"Data":{"/":{"bytes":"CAE"}},"Links":[]}' | ipfs dag put --store-codec dag-pb)

  test_expect_success "'ipfs object patch' should work (no unixfs-dir)" '
    OUTPUT=$(ipfs object patch $EMPTY_DIR add-link foo $EMPTY_DIR) &&
    ipfs dag stat $OUTPUT
  '

  test_expect_success "'ipfs object patch' should work" '
    OUTPUT=$(ipfs object patch $EMPTY_UNIXFS_DIR add-link foo $EMPTY_UNIXFS_DIR) &&
    ipfs dag stat $OUTPUT
  '

  test_expect_success "'ipfs object patch' check output block size" '
    DIR=$EMPTY_UNIXFS_DIR
    for i in {1..13}
    do
       DIR=$(ipfs object patch "$DIR" add-link "$DIR.jpg" "$DIR")
    done
    # Fail when new block goes over the BS limit of 1MiB, but allow manual override
    test_expect_code 1 ipfs object patch "$DIR" add-link "$DIR.jpg" "$DIR"  >patch_out 2>&1
  '

  test_expect_success "ipfs object patch add-link output has the correct error" '
    grep "produced block is over 1MiB" patch_out
  '

  test_expect_success "ipfs object patch --allow-big-block=true add-link works" '
    test_expect_code 0 ipfs object patch --allow-big-block=true "$DIR" add-link "$DIR.jpg" "$DIR"
  '

  test_expect_success "'ipfs object patch add-link' should work with paths" '
    N1=$(ipfs object patch $EMPTY_UNIXFS_DIR add-link baz $EMPTY_UNIXFS_DIR) &&
    N2=$(ipfs object patch $EMPTY_UNIXFS_DIR add-link bar $N1) &&
    N3=$(ipfs object patch $EMPTY_UNIXFS_DIR add-link foo /ipfs/$N2/bar) &&
    ipfs dag stat /ipfs/$N3 > /dev/null &&
    ipfs dag stat $N3/foo > /dev/null &&
    ipfs dag stat /ipfs/$N3/foo/baz > /dev/null
  '

  test_expect_success "'ipfs object patch add-link' allow linking IPLD objects" '
    OBJ=$(echo "123" | ipfs dag put) &&
    N1=$(ipfs object patch $EMPTY_UNIXFS_DIR add-link foo $OBJ) &&

    ipfs dag stat /ipfs/$N1 > /dev/null &&
    ipfs resolve /ipfs/$N1/foo > actual  &&
    echo /ipfs/$OBJ > expected &&

    test_cmp expected actual
  '

  test_expect_success "object patch creation looks right" '
    echo "bafybeiakusqwohnt7bs75kx6jhmt4oi47l634bmudxfv4qxhpco6xuvgna" > hash_exp &&
    echo $N3 > hash_actual &&
    test_cmp hash_exp hash_actual
  '

  test_expect_success "multilayer ipfs patch works" '
    echo "hello world" > hwfile &&
    FILE=$(ipfs add -q hwfile) &&
    EMPTY=$EMPTY_UNIXFS_DIR &&
    ONE=$(ipfs object patch $EMPTY add-link b $EMPTY) &&
    TWO=$(ipfs object patch $EMPTY add-link a $ONE) &&
    ipfs object patch $TWO add-link a/b/c $FILE > multi_patch
  '

  test_expect_success "output looks good" '
    ipfs cat $(cat multi_patch)/a/b/c > hwfile_out &&
    test_cmp hwfile hwfile_out
  '

  test_expect_success "can remove the directory" '
    ipfs object patch $OUTPUT rm-link foo > rmlink_output
  '

  test_expect_success "output should be empty" '
    echo bafybeiczsscdsbs7ffqz55asqdf3smv6klcw3gofszvwlyarci47bgf354 > rmlink_exp &&
    test_cmp rmlink_exp rmlink_output
  '

  test_expect_success "multilayer rm-link should work" '
    ipfs object patch $(cat multi_patch) rm-link a/b/c > multi_link_rm_out
  '

  test_expect_success "output looks good" '
    echo "bafybeicourxysmtbe5hacxqico4d5hyvh7gqkrwlmqa4ew7zufn3pj3juu" > multi_link_rm_exp &&
    test_cmp multi_link_rm_exp multi_link_rm_out
  '

  test_patch_create_path $EMPTY a/b/c $FILE

  test_patch_create_path $EMPTY a $FILE

  test_patch_create_path $EMPTY a/b/b/b/b $FILE

  test_expect_success "can create blank object" '
    BLANK=$EMPTY_DIR
  '

  test_patch_create_path $BLANK a $FILE

  test_expect_success "create bad path fails" '
    test_must_fail ipfs object patch $EMPTY add-link --create / $FILE
  '
}

# should work offline
test_object_cmd

# should work online
test_launch_ipfs_daemon
test_object_cmd
test_kill_ipfs_daemon

test_done
