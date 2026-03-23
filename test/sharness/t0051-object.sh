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
  # Bare dag-pb node with no UnixFS metadata (0 bytes of protobuf data)
  EMPTY_DIR=$(echo '{"Links":[]}' | ipfs dag put --store-codec dag-pb)
  # Empty UnixFS directory (equivalent to QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn)
  EMPTY_UNIXFS_DIR=$(echo '{"Data":{"/":{"bytes":"CAE"}},"Links":[]}' | ipfs dag put --store-codec dag-pb)
  # Empty UnixFS file (QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH)
  EMPTY_UNIXFS_FILE=$(echo -n | ipfs add -q)
  # Empty HAMTShard (Type=HAMTShard, HashType=0x22, Fanout=256)
  EMPTY_HAMT=$(echo '{"Data":{"/":{"bytes":"CAUoIjCAAg"}},"Links":[]}' | ipfs dag put --store-codec dag-pb)

  # --- UnixFS validation for 'object patch add-link' ---
  # 'object patch' operates at the dag-pb level via dagutils.Editor, which
  # only manipulates ProtoNode links without updating UnixFS metadata.
  # Only plain UnixFS Directory nodes are safe to mutate this way.
  # https://specs.ipfs.tech/unixfs/#pbnode-links-name
  # https://github.com/ipfs/kubo/issues/7190
  #
  # Four root node types tested below:
  #   1) bare dag-pb (no UnixFS data)  -- rejected
  #   2) UnixFS File                   -- rejected (prevents data loss)
  #   3) HAMTShard                     -- rejected (corrupts HAMT bitfield)
  #   4) UnixFS Directory              -- allowed

  # Reproduce https://github.com/ipfs/kubo/issues/7190:
  # adding a named link to a File node must be rejected to prevent data loss.
  test_expect_success "'ipfs object patch add-link' prevents data loss on File nodes (#7190)" '
    echo "original content" > original.txt &&
    ORIGINAL_CID=$(ipfs add -q original.txt) &&
    CHILD_CID=$(echo "child" | ipfs add -q) &&
    test_expect_code 1 ipfs object patch $ORIGINAL_CID add-link "child.txt" $CHILD_CID 2>patch_7190_err &&
    echo "Error: cannot add named links to a UnixFS File node, only Directory nodes support link addition at the dag-pb level (see https://specs.ipfs.tech/unixfs/)" >patch_7190_expected &&
    test_cmp patch_7190_expected patch_7190_err &&
    # verify the original file is still intact
    ipfs cat $ORIGINAL_CID > original_readback.txt &&
    test_cmp original.txt original_readback.txt
  '

  # 1) Bare dag-pb (no UnixFS data): rejected by default
  test_expect_success "'ipfs object patch add-link' rejects non-UnixFS dag-pb nodes" '
    test_expect_code 1 ipfs object patch $EMPTY_DIR add-link foo $EMPTY_UNIXFS_DIR 2>patch_dagpb_err
  '

  test_expect_success "add-link error for non-UnixFS dag-pb has expected message" '
    echo "Error: cannot add named links to a non-UnixFS dag-pb node; pass --allow-non-unixfs to skip validation" >patch_dagpb_expected &&
    test_cmp patch_dagpb_expected patch_dagpb_err
  '

  test_expect_success "'ipfs object patch add-link --allow-non-unixfs' works on dag-pb nodes" '
    OUTPUT=$(ipfs object patch $EMPTY_DIR add-link --allow-non-unixfs foo $EMPTY_UNIXFS_DIR) &&
    ipfs dag stat $OUTPUT
  '

  # 2) UnixFS File (QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH): rejected by default
  test_expect_success "'ipfs object patch add-link' rejects UnixFS File nodes" '
    test_expect_code 1 ipfs object patch $EMPTY_UNIXFS_FILE add-link foo $EMPTY_UNIXFS_DIR 2>patch_file_err
  '

  test_expect_success "add-link error for UnixFS File has expected message" '
    echo "Error: cannot add named links to a UnixFS File node, only Directory nodes support link addition at the dag-pb level (see https://specs.ipfs.tech/unixfs/)" >patch_file_expected &&
    test_cmp patch_file_expected patch_file_err
  '

  test_expect_success "'ipfs object patch add-link --allow-non-unixfs' bypasses check on File nodes" '
    ipfs object patch $EMPTY_UNIXFS_FILE add-link --allow-non-unixfs foo $EMPTY_UNIXFS_DIR
  '

  # 3) HAMTShard: rejected (dag-pb level mutation corrupts HAMT bitfield)
  test_expect_success "'ipfs object patch add-link' rejects HAMTShard nodes" '
    test_expect_code 1 ipfs object patch $EMPTY_HAMT add-link foo $EMPTY_UNIXFS_DIR 2>patch_hamt_err
  '

  test_expect_success "add-link error for HAMTShard has expected message" '
    echo "Error: cannot add links to a HAMTShard at the dag-pb level (would corrupt the HAMT bitfield); use '"'"'ipfs files'"'"' commands instead, or pass --allow-non-unixfs to override" >patch_hamt_expected &&
    test_cmp patch_hamt_expected patch_hamt_err
  '

  test_expect_success "'ipfs object patch add-link --allow-non-unixfs' bypasses check on HAMTShard" '
    ipfs object patch $EMPTY_HAMT add-link --allow-non-unixfs foo $EMPTY_UNIXFS_DIR
  '

  # 4) UnixFS Directory (QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn): allowed
  test_expect_success "'ipfs object patch add-link' works on UnixFS Directory nodes" '
    OUTPUT=$(ipfs object patch $EMPTY_UNIXFS_DIR add-link foo $EMPTY_UNIXFS_DIR) &&
    ipfs dag stat $OUTPUT
  '

  test_expect_success "'ipfs object patch' check output block size" '
    DIR=$EMPTY_UNIXFS_DIR
    for i in {1..14}
    do
       DIR=$(ipfs object patch "$DIR" add-link "$DIR.jpg" "$DIR")
    done
    # Fail when new block goes over the BS limit of 2MiB, but allow manual override
    test_expect_code 1 ipfs object patch "$DIR" add-link "$DIR.jpg" "$DIR"  >patch_out 2>&1
  '

  test_expect_success "ipfs object patch add-link output has the correct error" '
    grep "produced block is over 2MiB" patch_out
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

  test_expect_success "'ipfs object patch add-link --create' rejects non-UnixFS roots" '
    test_must_fail ipfs object patch $EMPTY_DIR add-link --create a $FILE
  '

  test_expect_success "'ipfs object patch add-link --create --allow-non-unixfs' works on non-UnixFS roots" '
    PCOUT=$(ipfs object patch $EMPTY_DIR add-link --create --allow-non-unixfs a $FILE) &&
    ipfs cat "$PCOUT/a" >tpcp_out &&
    ipfs cat "$FILE" >tpcp_exp &&
    test_cmp tpcp_exp tpcp_out
  '

  test_expect_success "create bad path fails" '
    test_must_fail ipfs object patch $EMPTY add-link --create / $FILE
  '

  # --- UnixFS validation for 'object patch rm-link' ---
  # Same rationale as add-link: dagutils.Editor cannot update UnixFS metadata.

  # 1) Bare dag-pb: rejected by default
  test_expect_success "'ipfs object patch rm-link' rejects non-UnixFS dag-pb nodes" '
    DAGPB_WITH_LINK=$(ipfs object patch $EMPTY_DIR add-link --allow-non-unixfs foo $EMPTY_UNIXFS_DIR) &&
    test_expect_code 1 ipfs object patch $DAGPB_WITH_LINK rm-link foo 2>rmlink_dagpb_err
  '

  test_expect_success "rm-link error for non-UnixFS dag-pb has expected message" '
    echo "Error: cannot remove links from a non-UnixFS dag-pb node; pass --allow-non-unixfs to skip validation" >rmlink_dagpb_expected &&
    test_cmp rmlink_dagpb_expected rmlink_dagpb_err
  '

  test_expect_success "'ipfs object patch rm-link --allow-non-unixfs' works on dag-pb nodes" '
    ipfs object patch $DAGPB_WITH_LINK rm-link --allow-non-unixfs foo
  '

  # 2) UnixFS File: rejected by default
  test_expect_success "'ipfs object patch rm-link' rejects UnixFS File nodes" '
    FILE_WITH_LINK=$(ipfs object patch $EMPTY_UNIXFS_FILE add-link --allow-non-unixfs foo $EMPTY_UNIXFS_DIR) &&
    test_expect_code 1 ipfs object patch $FILE_WITH_LINK rm-link foo 2>rmlink_file_err
  '

  test_expect_success "rm-link error for UnixFS File has expected message" '
    echo "Error: cannot remove links from a UnixFS File node, only Directory nodes support link removal at the dag-pb level (see https://specs.ipfs.tech/unixfs/)" >rmlink_file_expected &&
    test_cmp rmlink_file_expected rmlink_file_err
  '

  test_expect_success "'ipfs object patch rm-link --allow-non-unixfs' bypasses check on File nodes" '
    ipfs object patch $FILE_WITH_LINK rm-link --allow-non-unixfs foo
  '

  # 3) HAMTShard: rejected by default
  test_expect_success "'ipfs object patch rm-link' rejects HAMTShard nodes" '
    HAMT_WITH_LINK=$(ipfs object patch $EMPTY_HAMT add-link --allow-non-unixfs foo $EMPTY_UNIXFS_DIR) &&
    test_expect_code 1 ipfs object patch $HAMT_WITH_LINK rm-link foo 2>rmlink_hamt_err
  '

  test_expect_success "rm-link error for HAMTShard has expected message" '
    echo "Error: cannot remove links from a HAMTShard at the dag-pb level (would corrupt the HAMT bitfield); use '"'"'ipfs files rm'"'"' instead, or pass --allow-non-unixfs to override" >rmlink_hamt_expected &&
    test_cmp rmlink_hamt_expected rmlink_hamt_err
  '

  test_expect_success "'ipfs object patch rm-link --allow-non-unixfs' bypasses check on HAMTShard" '
    ipfs object patch $HAMT_WITH_LINK rm-link --allow-non-unixfs foo
  '

  # 4) UnixFS Directory: allowed (already tested above in existing rm-link tests)
}

# should work offline
test_object_cmd

# should work online
test_launch_ipfs_daemon
test_object_cmd
test_kill_ipfs_daemon

test_done
