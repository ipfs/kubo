#!/usr/bin/env bash
#
# Copyright (c) 2017 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test out the filestore nocopy functionality"

. lib/test-lib.sh

test_init_filestore() {
  test_expect_success "clean up old node" '
    rm -rf "$IPFS_PATH" mountdir ipfs ipns mfs
  '

  test_init_ipfs

  test_expect_success "enable filestore config setting" '
    ipfs config --json Experimental.FilestoreEnabled true
  '
}

test_init_dataset() {
  test_expect_success "create a dataset" '
    rm -r somedir
    mkdir somedir &&
    random-data -size=1000     -seed=1 > somedir/file1 &&
    random-data -size=10000    -seed=2 > somedir/file2 &&
    random-data -size=1000000  -seed=3 > somedir/file3
  '
}

test_init() {
  test_init_filestore
  test_init_dataset
}

EXPHASH="QmXqfraAT3U8ct14PPPXcFkWyvmqUZazLdo29GXTKSHkP4"

cat <<EOF > ls_expect_file_order
bafkreidx7ivgllulfkzyoo4oa7dfrg4mjmudg2qgdivoooj4s7lh3m5nqu   1000 somedir/file1 0
bafkreic2wqrsyr3y3qgzbvufen2w25r3p3zljckqyxkpcagsxz3zdcosd4  10000 somedir/file2 0
bafkreiemzfmzws23c2po4m6deiueknqfty7r3voes3e3zujmobrooc2ngm 262144 somedir/file3 0
bafkreihgm53yhxn427lnfdwhqgpawc62qejog7gega5kqb6uwbyhjm47hu 262144 somedir/file3 262144
bafkreigl2pjptgxz6cexcnua56zc5dwsyrc4ph2eulmcb634oes6gzvmuy 262144 somedir/file3 524288
bafkreifjcthslybjizk36xffcsb32fsbguxz3ptkl7723wz4u3qikttmam 213568 somedir/file3 786432
EOF

sort < ls_expect_file_order > ls_expect_key_order

FILE1_HASH=bafkreidx7ivgllulfkzyoo4oa7dfrg4mjmudg2qgdivoooj4s7lh3m5nqu
FILE2_HASH=bafkreic2wqrsyr3y3qgzbvufen2w25r3p3zljckqyxkpcagsxz3zdcosd4
FILE3_HASH=QmYEZtRGGk8rgM8MetegLLRHMKskPCg7zWpmQQAo3cQiN5

cat <<EOF > verify_expect_file_order
ok      bafkreidx7ivgllulfkzyoo4oa7dfrg4mjmudg2qgdivoooj4s7lh3m5nqu   1000 somedir/file1 0
ok      bafkreic2wqrsyr3y3qgzbvufen2w25r3p3zljckqyxkpcagsxz3zdcosd4  10000 somedir/file2 0
ok      bafkreiemzfmzws23c2po4m6deiueknqfty7r3voes3e3zujmobrooc2ngm 262144 somedir/file3 0
ok      bafkreihgm53yhxn427lnfdwhqgpawc62qejog7gega5kqb6uwbyhjm47hu 262144 somedir/file3 262144
ok      bafkreigl2pjptgxz6cexcnua56zc5dwsyrc4ph2eulmcb634oes6gzvmuy 262144 somedir/file3 524288
ok      bafkreifjcthslybjizk36xffcsb32fsbguxz3ptkl7723wz4u3qikttmam 213568 somedir/file3 786432
EOF

sort < verify_expect_file_order > verify_expect_key_order

cat <<EOF > verify_rm_expect
ok      bafkreic2wqrsyr3y3qgzbvufen2w25r3p3zljckqyxkpcagsxz3zdcosd4  10000 somedir/file2 0 keep
ok      bafkreidx7ivgllulfkzyoo4oa7dfrg4mjmudg2qgdivoooj4s7lh3m5nqu   1000 somedir/file1 0 keep
changed bafkreiemzfmzws23c2po4m6deiueknqfty7r3voes3e3zujmobrooc2ngm 262144 somedir/file3 0 remove
changed bafkreifjcthslybjizk36xffcsb32fsbguxz3ptkl7723wz4u3qikttmam 213568 somedir/file3 786432 remove
changed bafkreigl2pjptgxz6cexcnua56zc5dwsyrc4ph2eulmcb634oes6gzvmuy 262144 somedir/file3 524288 remove
changed bafkreihgm53yhxn427lnfdwhqgpawc62qejog7gega5kqb6uwbyhjm47hu 262144 somedir/file3 262144 remove
EOF

cat <<EOF > verify_after_rm_expect
ok      bafkreic2wqrsyr3y3qgzbvufen2w25r3p3zljckqyxkpcagsxz3zdcosd4  10000 somedir/file2 0
ok      bafkreidx7ivgllulfkzyoo4oa7dfrg4mjmudg2qgdivoooj4s7lh3m5nqu   1000 somedir/file1 0
EOF

IPFS_CMD="ipfs"

test_filestore_adds() {
  test_expect_success "$IPFS_CMD add nocopy add succeeds" '
    HASH=$($IPFS_CMD add --raw-leaves --nocopy -r -Q somedir)
  '

  test_expect_success "nocopy add has right hash" '
    test "$HASH" = "$EXPHASH"
  '

  test_expect_success "'$IPFS_CMD filestore ls' output looks good'" '
    $IPFS_CMD filestore ls | sort > ls_actual &&
    test_cmp ls_expect_key_order ls_actual
  '

  test_expect_success "'$IPFS_CMD filestore ls --file-order' output looks good'" '
    $IPFS_CMD filestore ls --file-order > ls_actual &&
    test_cmp ls_expect_file_order ls_actual
  '

  test_expect_success "'$IPFS_CMD filestore ls HASH' works" '
    $IPFS_CMD filestore ls $FILE1_HASH > ls_actual &&
    grep -q somedir/file1 ls_actual
  '

  test_expect_success "can retrieve multi-block file" '
    $IPFS_CMD cat $FILE3_HASH > file3.data &&
    test_cmp somedir/file3 file3.data
  '
}

# check that the filestore is in a clean state
test_filestore_state() {
  test_expect_success "$IPFS_CMD filestore verify' output looks good'" '
    $IPFS_CMD filestore verify | LC_ALL=C sort > verify_actual
    test_cmp verify_expect_key_order verify_actual
  '
}

test_filestore_verify() {
  test_filestore_state

  test_expect_success "$IPFS_CMD filestore verify --file-order' output looks good'" '
    $IPFS_CMD filestore verify --file-order > verify_actual
    test_cmp verify_expect_file_order verify_actual
  '

  test_expect_success "'$IPFS_CMD filestore verify HASH' works" '
    $IPFS_CMD filestore verify $FILE1_HASH > verify_actual &&
    grep -q somedir/file1 verify_actual
  '

  test_expect_success "rename a file" '
    mv somedir/file1 somedir/file1.bk
  '

  test_expect_success "can not retrieve block after backing file moved" '
    test_must_fail $IPFS_CMD cat $FILE1_HASH
  '

  test_expect_success "'$IPFS_CMD filestore verify' shows file as missing" '
    $IPFS_CMD filestore verify > verify_actual &&
    grep no-file verify_actual | grep -q somedir/file1
  '

  test_expect_success "move file back" '
    mv somedir/file1.bk somedir/file1
  '

  test_expect_success "block okay now" '
    $IPFS_CMD cat $FILE1_HASH > file1.data &&
    test_cmp somedir/file1 file1.data
  '

  test_expect_success "change first bit of file" '
    dd if=/dev/zero of=somedir/file3 bs=1024 count=1
  '

  test_expect_success "can not retrieve block after backing file changed" '
    test_must_fail $IPFS_CMD cat $FILE3_HASH
  '

  test_expect_success "'$IPFS_CMD filestore verify' shows file as changed" '
    $IPFS_CMD filestore verify > verify_actual &&
    grep changed verify_actual | grep -q somedir/file3
  '

  # reset the state for the next test
  test_init_dataset
}

test_filestore_rm_bad_blocks() {
  test_filestore_state

  test_expect_success "change first bit of file" '
    dd if=/dev/zero of=somedir/file3 bs=1024 count=1
  '

  test_expect_success "'$IPFS_CMD filestore verify --remove-bad-blocks' shows changed file removed" '
    $IPFS_CMD filestore verify --remove-bad-blocks > verify_rm_actual &&
    test_cmp verify_rm_expect verify_rm_actual
  '

  test_expect_success "'$IPFS_CMD filestore verify' shows only files that were not removed" '
    $IPFS_CMD filestore verify > verify_after &&
    test_cmp verify_after_rm_expect verify_after
  '
  
  # reset the state for the next test
  test_init_dataset
}
  
test_filestore_dups() {
  # make sure the filestore is in a clean state
  test_filestore_state

  test_expect_success "'$IPFS_CMD filestore dups'" '
    $IPFS_CMD add --raw-leaves somedir/file1 &&
    $IPFS_CMD filestore dups > dups_actual &&
    echo "$FILE1_HASH" > dups_expect
    test_cmp dups_expect dups_actual
  '
}

#
# No daemon
#

test_init

test_filestore_adds

test_filestore_verify

test_filestore_dups

test_filestore_rm_bad_blocks

#
# With daemon
#

test_init

# must be in offline mode so tests that retrieve non-existent blocks
# doesn't hang
test_launch_ipfs_daemon_without_network

test_filestore_adds

test_filestore_verify

test_filestore_dups

test_kill_ipfs_daemon

test_filestore_rm_bad_blocks

##
## base32
##

EXPHASH="bafybeienfbjfbywu5y44i5qm4wxajblgy5a6xuc4eepjaw5fq223wwsy3m"

cat <<EOF > ls_expect_file_order
bafkreidx7ivgllulfkzyoo4oa7dfrg4mjmudg2qgdivoooj4s7lh3m5nqu   1000 somedir/file1 0
bafkreic2wqrsyr3y3qgzbvufen2w25r3p3zljckqyxkpcagsxz3zdcosd4  10000 somedir/file2 0
bafkreiemzfmzws23c2po4m6deiueknqfty7r3voes3e3zujmobrooc2ngm 262144 somedir/file3 0
bafkreihgm53yhxn427lnfdwhqgpawc62qejog7gega5kqb6uwbyhjm47hu 262144 somedir/file3 262144
bafkreigl2pjptgxz6cexcnua56zc5dwsyrc4ph2eulmcb634oes6gzvmuy 262144 somedir/file3 524288
bafkreifjcthslybjizk36xffcsb32fsbguxz3ptkl7723wz4u3qikttmam 213568 somedir/file3 786432
EOF

sort < ls_expect_file_order > ls_expect_key_order

FILE1_HASH=bafkreidx7ivgllulfkzyoo4oa7dfrg4mjmudg2qgdivoooj4s7lh3m5nqu
FILE2_HASH=bafkreic2wqrsyr3y3qgzbvufen2w25r3p3zljckqyxkpcagsxz3zdcosd4
FILE3_HASH=bafybeietaxxjghilcjhc2m4zcmicm7yjvkjdfkamc3ct2hq4gmsb3shqsi

cat <<EOF > verify_expect_file_order
ok      bafkreidx7ivgllulfkzyoo4oa7dfrg4mjmudg2qgdivoooj4s7lh3m5nqu   1000 somedir/file1 0
ok      bafkreic2wqrsyr3y3qgzbvufen2w25r3p3zljckqyxkpcagsxz3zdcosd4  10000 somedir/file2 0
ok      bafkreiemzfmzws23c2po4m6deiueknqfty7r3voes3e3zujmobrooc2ngm 262144 somedir/file3 0
ok      bafkreihgm53yhxn427lnfdwhqgpawc62qejog7gega5kqb6uwbyhjm47hu 262144 somedir/file3 262144
ok      bafkreigl2pjptgxz6cexcnua56zc5dwsyrc4ph2eulmcb634oes6gzvmuy 262144 somedir/file3 524288
ok      bafkreifjcthslybjizk36xffcsb32fsbguxz3ptkl7723wz4u3qikttmam 213568 somedir/file3 786432
EOF

sort < verify_expect_file_order > verify_expect_key_order

IPFS_CMD="ipfs --cid-base=base32"

#
# No daemon
#

test_init

test_filestore_adds

test_filestore_verify

test_filestore_dups

test_filestore_rm_bad_blocks

#
# With daemon
#

test_init

# must be in offline mode so tests that retrieve non-existent blocks
# doesn't hang
test_launch_ipfs_daemon_without_network

test_filestore_adds

test_filestore_verify

test_filestore_dups

test_kill_ipfs_daemon

test_done

test_filestore_rm_bad_blocks

##

test_done
