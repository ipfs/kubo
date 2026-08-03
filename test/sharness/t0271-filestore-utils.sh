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

EXPHASH="QmT8nut4Y5pHXUftaX7PC7QcojAQfYsjiLMoxwdc7bNS7p"

cat <<EOF > ls_expect_file_order
bafkreihed5ipyj3uenaem5zroilgdrhp4lvs7c2r6ldgcd7is66xj6izmi   1000 somedir/file1 0
bafkreidfpbh7afcrsl4bc67qnmrtiksdqoxdto7tug5zo7bkh3vq46svs4  10000 somedir/file2 0
bafkreihj5dckiavi3e3hpyxy544aewbne3gca2synmo6bpgmx6ezitklyq 262144 somedir/file3 0
bafkreigfefue3rninmokrkyuho5dvrj7sjfvzunztmqusnfms4jpjkg3sq 262144 somedir/file3 262144
bafkreid4ozonym3z5tm5rm2cli5fmzudkcwnazzhbgu2k6kpbj5qj27mha 262144 somedir/file3 524288
bafkreiap7vddw56ggaod7d7kot4s3tvcc6aidlyoqzp4wuqp7c63zsttw4 213568 somedir/file3 786432
EOF

sort < ls_expect_file_order > ls_expect_key_order

FILE1_HASH=bafkreihed5ipyj3uenaem5zroilgdrhp4lvs7c2r6ldgcd7is66xj6izmi
FILE2_HASH=bafkreidfpbh7afcrsl4bc67qnmrtiksdqoxdto7tug5zo7bkh3vq46svs4
FILE3_HASH=QmWtHwhJ3RKxJpdv7M99MmF2g2G882LKLKFphYmCnaWre2

cat <<EOF > verify_expect_file_order
ok      bafkreihed5ipyj3uenaem5zroilgdrhp4lvs7c2r6ldgcd7is66xj6izmi   1000 somedir/file1 0
ok      bafkreidfpbh7afcrsl4bc67qnmrtiksdqoxdto7tug5zo7bkh3vq46svs4  10000 somedir/file2 0
ok      bafkreihj5dckiavi3e3hpyxy544aewbne3gca2synmo6bpgmx6ezitklyq 262144 somedir/file3 0
ok      bafkreigfefue3rninmokrkyuho5dvrj7sjfvzunztmqusnfms4jpjkg3sq 262144 somedir/file3 262144
ok      bafkreid4ozonym3z5tm5rm2cli5fmzudkcwnazzhbgu2k6kpbj5qj27mha 262144 somedir/file3 524288
ok      bafkreiap7vddw56ggaod7d7kot4s3tvcc6aidlyoqzp4wuqp7c63zsttw4 213568 somedir/file3 786432
EOF

sort < verify_expect_file_order > verify_expect_key_order

cat <<EOF > verify_rm_expect
changed bafkreiap7vddw56ggaod7d7kot4s3tvcc6aidlyoqzp4wuqp7c63zsttw4 213568 somedir/file3 786432 remove
ok      bafkreidfpbh7afcrsl4bc67qnmrtiksdqoxdto7tug5zo7bkh3vq46svs4  10000 somedir/file2 0 keep
changed bafkreid4ozonym3z5tm5rm2cli5fmzudkcwnazzhbgu2k6kpbj5qj27mha 262144 somedir/file3 524288 remove
changed bafkreigfefue3rninmokrkyuho5dvrj7sjfvzunztmqusnfms4jpjkg3sq 262144 somedir/file3 262144 remove
ok      bafkreihed5ipyj3uenaem5zroilgdrhp4lvs7c2r6ldgcd7is66xj6izmi   1000 somedir/file1 0 keep
changed bafkreihj5dckiavi3e3hpyxy544aewbne3gca2synmo6bpgmx6ezitklyq 262144 somedir/file3 0 remove
EOF

cat <<EOF > verify_after_rm_expect
ok      bafkreidfpbh7afcrsl4bc67qnmrtiksdqoxdto7tug5zo7bkh3vq46svs4  10000 somedir/file2 0
ok      bafkreihed5ipyj3uenaem5zroilgdrhp4lvs7c2r6ldgcd7is66xj6izmi   1000 somedir/file1 0
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

EXPHASH="bafybeichic6xnxoz63r5kk736bxskyfstt4bdqiqlmfy3kkp7modsf2v64"

cat <<EOF > ls_expect_file_order
bafkreihed5ipyj3uenaem5zroilgdrhp4lvs7c2r6ldgcd7is66xj6izmi   1000 somedir/file1 0
bafkreidfpbh7afcrsl4bc67qnmrtiksdqoxdto7tug5zo7bkh3vq46svs4  10000 somedir/file2 0
bafkreihj5dckiavi3e3hpyxy544aewbne3gca2synmo6bpgmx6ezitklyq 262144 somedir/file3 0
bafkreigfefue3rninmokrkyuho5dvrj7sjfvzunztmqusnfms4jpjkg3sq 262144 somedir/file3 262144
bafkreid4ozonym3z5tm5rm2cli5fmzudkcwnazzhbgu2k6kpbj5qj27mha 262144 somedir/file3 524288
bafkreiap7vddw56ggaod7d7kot4s3tvcc6aidlyoqzp4wuqp7c63zsttw4 213568 somedir/file3 786432
EOF

sort < ls_expect_file_order > ls_expect_key_order

FILE1_HASH=bafkreihed5ipyj3uenaem5zroilgdrhp4lvs7c2r6ldgcd7is66xj6izmi
FILE2_HASH=bafkreidfpbh7afcrsl4bc67qnmrtiksdqoxdto7tug5zo7bkh3vq46svs4
FILE3_HASH=bafybeid67cm7p4m75octeh54es3un5me5hr7udz6wchlek2mfz3tfllmx4

cat <<EOF > verify_expect_file_order
ok      bafkreihed5ipyj3uenaem5zroilgdrhp4lvs7c2r6ldgcd7is66xj6izmi   1000 somedir/file1 0
ok      bafkreidfpbh7afcrsl4bc67qnmrtiksdqoxdto7tug5zo7bkh3vq46svs4  10000 somedir/file2 0
ok      bafkreihj5dckiavi3e3hpyxy544aewbne3gca2synmo6bpgmx6ezitklyq 262144 somedir/file3 0
ok      bafkreigfefue3rninmokrkyuho5dvrj7sjfvzunztmqusnfms4jpjkg3sq 262144 somedir/file3 262144
ok      bafkreid4ozonym3z5tm5rm2cli5fmzudkcwnazzhbgu2k6kpbj5qj27mha 262144 somedir/file3 524288
ok      bafkreiap7vddw56ggaod7d7kot4s3tvcc6aidlyoqzp4wuqp7c63zsttw4 213568 somedir/file3 786432
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
