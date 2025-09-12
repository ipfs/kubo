#!/usr/bin/env bash

test_description="Test storing and retrieving mode and mtime"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "set Import defaults to ensure deterministic cids for mod and mtime tests" '
  ipfs config --json Import.CidVersion 0 &&
  ipfs config Import.HashFunction sha2-256 &&
  ipfs config Import.UnixFSChunker size-262144
'

HASH_NO_PRESERVE=QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH

PRESERVE_MTIME=1604320482
PRESERVE_MODE="0640"
HASH_PRESERVE_MODE=QmQLgxypSNGNFTuUPGCecq6dDEjb6hNB5xSyVmP3cEuNtq
HASH_PRESERVE_MTIME=QmQ6kErEW8kztQFV8vbwNU8E4dmtGsYpRiboiLxUEwibvj
HASH_PRESERVE_LINK_MTIME=QmbJwotgtr84JxcnjpwJ86uZiyMoxbZuNH4YrdJMypkYaB
HASH_PRESERVE_MODE_AND_MTIME=QmYkvboLsvLFcSYmqVJRxvBdYRQLroLv9kELf3LRiCqBri

CUSTOM_MTIME=1603539720
CUSTOM_MTIME_NSECS=54321
CUSTOM_MODE="0764"
HASH_CUSTOM_MODE=QmchD3BN8TQ3RW6jPLxSaNkqvfuj7syKhzTRmL4EpyY1Nz
HASH_CUSTOM_MTIME=QmT3aY4avDcYXCWpU8CJzqUkW7YEuEsx36S8cTNoLcuK1B
HASH_CUSTOM_MTIME_NSECS=QmaKH8H5rXBUBCX4vdxi7ktGQEL7wejV7L9rX2qpZjwncz
HASH_CUSTOM_MODE_AND_MTIME=QmUkxrtBA8tPjwCYz1HrsoRfDz6NgKut3asVeHVQNH4C8L
HASH_CUSTOM_LINK_MTIME=QmV1Uot2gy4bhY9yvYiZxhhchhyYC6MKKoGV1XtWNmpCLe
HASH_CUSTOM_LINK_MTIME_NSECS=QmPHYCxYvvHj6VxiPNJ3kXxcPsnJLDYUJqsDJWjvytmrmY

mk_name() {
  tr -dc '[:alnum:]'</dev/urandom|head -c 16
}

mk_file() {
    mktemp -p "$SHARNESS_TRASH_DIRECTORY" "mk_file_${1}_XXXXXX"
}

mk_dir() {
    mktemp -d -p "$SHARNESS_TRASH_DIRECTORY" "mk_dir_${1}_XXXXXX"
}

# force umask for deterministic mode on files created via touch
# (https://github.com/orgs/community/discussions/40876, https://github.com/ipfs/kubo/pull/10478/#discussion_r1717515514)
umask 022

FIXTURESDIR="$(mk_dir fixtures)"

test_file() {
  local TESTFILE="$FIXTURESDIR/test$1.txt"
  local TESTLINK="$FIXTURESDIR/linkfile$1"

  touch "$TESTFILE"
  ln -s nothing "$TESTLINK"

  test_expect_success "feature on file has no effect when not used [$1]" '
    touch "$TESTFILE" &&
    HASH=$(ipfs add -q "$TESTFILE") &&
    test "$HASH_NO_PRESERVE" = "$HASH"
  '

  test_expect_success "can preserve file mode [$1]" '
    touch "$TESTFILE" &&
    chmod $PRESERVE_MODE "$TESTFILE" &&
    HASH=$(ipfs add -q --preserve-mode "$TESTFILE") &&
    test "$HASH_PRESERVE_MODE" = "$HASH"
  '

  test_expect_success "can preserve file modification time [$1]" '
    touch -m -d @$PRESERVE_MTIME "$TESTFILE" &&
    HASH=$(ipfs add -q --preserve-mtime "$TESTFILE") &&
    test "$HASH_PRESERVE_MTIME" = "$HASH"
  '

  test_expect_success "can preserve file mode and modification time [$1]" '
    touch -m -d @$PRESERVE_MTIME "$TESTFILE" &&
    chmod $PRESERVE_MODE "$TESTFILE" &&
    HASH=$(ipfs add -q --preserve-mode --preserve-mtime "$TESTFILE") &&
    test "$HASH_PRESERVE_MODE_AND_MTIME" = "$HASH"
  '

  test_expect_success "can preserve symlink modification time [$1]" '
    touch -h -m -d @$PRESERVE_MTIME "$TESTLINK" &&
    HASH=$(ipfs add -q --preserve-mtime "$TESTLINK") &&
    test "$HASH_PRESERVE_LINK_MTIME" = "$HASH"
  '

  test_expect_success "can set file mode [$1]" '
    touch "$TESTFILE" &&
    chmod 0600 "$TESTFILE" &&
    HASH=$(ipfs add -q --mode=$CUSTOM_MODE "$TESTFILE") &&
    test "$HASH_CUSTOM_MODE" = "$HASH"
  '

  test_expect_success "can set file modification time [$1]" '
    touch -m -t 202011021234.42 "$TESTFILE" &&
    HASH=$(ipfs add -q --mtime=$CUSTOM_MTIME "$TESTFILE") &&
    test "$HASH_CUSTOM_MTIME" = "$HASH"
  '

  test_expect_success "can set file modification time nanoseconds [$1]" '
    touch -m -t 202011021234.42 "$TESTFILE" &&
    HASH=$(ipfs add -q --mtime=$CUSTOM_MTIME --mtime-nsecs=$CUSTOM_MTIME_NSECS "$TESTFILE") &&
    test "$HASH_CUSTOM_MTIME_NSECS" = "$HASH"
  '

  test_expect_success "can set file mode and modification time [$1]" '
    touch -m -t 202011021234.42 "$TESTFILE" &&
    chmod 0600 "$TESTFILE" &&
    HASH=$(ipfs add -q --mode=$CUSTOM_MODE --mtime=$CUSTOM_MTIME --mtime-nsecs=$CUSTOM_MTIME_NSECS "$TESTFILE") &&
    test "$HASH_CUSTOM_MODE_AND_MTIME" = "$HASH"
  '

  test_expect_success "can set symlink modification time [$1]" '
    touch -h -m -t 202011021234.42 "$TESTLINK" &&
    HASH=$(ipfs add -q --mtime=$CUSTOM_MTIME "$TESTLINK") &&
    test "$HASH_CUSTOM_LINK_MTIME" = "$HASH"
  '

  test_expect_success "cannot set mode on symbolic link" '
    HASH=$(ipfs add -q --mtime=$CUSTOM_MTIME --mode=$CUSTOM_MODE "$TESTLINK") &&
    ACTUAL=$(ipfs files stat --format="<mode>" /ipfs/$HASH) &&
    test "$ACTUAL" = "lrwxrwxrwx"
  '


  test_expect_success "can set symlink modification time nanoseconds [$1]" '
    touch -h -m -t 202011021234.42 "$TESTLINK" &&
    HASH=$(ipfs add -q --mtime=$CUSTOM_MTIME --mtime-nsecs=$CUSTOM_MTIME_NSECS "$TESTLINK") &&
    test "$HASH_CUSTOM_LINK_MTIME_NSECS" = "$HASH"
  '

  test_expect_success "can get preserved mode and modification time [$1]" '
    OUTFILE="$(mk_file $HASH_PRESERVE_MODE_AND_MTIME)" &&
    ipfs get -o "$OUTFILE" $HASH_PRESERVE_MODE_AND_MTIME &&
    test "$PRESERVE_MODE:$PRESERVE_MTIME" = "$(stat -c "0%a:%Y" "$OUTFILE")"
  '

  test_expect_success "can get custom mode and modification time [$1]" '
    OUTFILE="$(mk_file $HASH_CUSTOM_MODE_AND_MTIME)" &&
    ipfs get -o "$OUTFILE" $HASH_CUSTOM_MODE_AND_MTIME &&
    TIMESTAMP=$(date +%s%N --date="$(stat -c "%y" "$OUTFILE")") &&
    MODETIME=$(stat -c "0%a:$TIMESTAMP" "$OUTFILE") &&
    printf -v EXPECTED "$CUSTOM_MODE:$CUSTOM_MTIME%09d" $CUSTOM_MTIME_NSECS &&
    test "$EXPECTED" = "$MODETIME"
  '

  test_expect_success "can get custom symlink modification time [$1]" '
    OUTFILE="$(mk_file $HASH_CUSTOM_LINK_MTIME_NSECS)" &&
    ipfs get -o "$OUTFILE" $HASH_CUSTOM_LINK_MTIME_NSECS &&
    TIMESTAMP=$(date +%s%N --date="$(stat -c "%y" "$OUTFILE")") &&
    printf -v EXPECTED "$CUSTOM_MTIME%09d" $CUSTOM_MTIME_NSECS &&
    test "$EXPECTED" = "$TIMESTAMP"
  '

  test_expect_success "can change file mode [$1]" '
    NAME=$(mk_name) &&
    HASH=$(echo testfile | ipfs add -q --mode=0600) &&
    OUTFILE=$(mk_file "${NAME}") &&
    ipfs files cp "/ipfs/$HASH" /$NAME &&
    ipfs files chmod 444 /$NAME &&
    HASH2=$(ipfs files stat /$NAME|head -1) &&
    ipfs get -o "$OUTFILE" $HASH2 &&
    test $(stat -c "%a" "$OUTFILE") = 444
  '

  # special case, because storing mode requires dag-pb envelope
  # and when dealing with CIDv1 we can have 'raw' block instead of 'dag-pb'
  # so it needs to be converted before adding attribute
  test_expect_success "can add file mode to cidv1 raw block [$1]" '
    NAME=$(mk_name) &&
    HASH=$(date | ipfs add -q --cid-version 1 --raw-leaves=true) &&
    OUTFILE=$(mk_file "${NAME}") &&
    ipfs files cp "/ipfs/$HASH" /$NAME &&
    ipfs files chmod 445 /$NAME &&
    HASH2=$(ipfs files stat /$NAME|head -1) &&
    ipfs get -o "$OUTFILE" $HASH2 &&
    test $(stat -c "%a" "$OUTFILE") = 445
  '

  test_expect_success "can change file modification time [$1]" '
    NAME=$(mk_name) &&
    OUTFILE="$(mk_file "$NAME")" &&
    NOW=$(date +%s) &&
    HASH=$(echo testfile | ipfs add -q --mtime=$NOW) &&
    ipfs files cp "/ipfs/$HASH" /$NAME &&
    sleep 1 &&
    ipfs files touch /$NAME &&
    HASH=$(ipfs files stat /$NAME|head -1) &&
    ipfs get -o "$OUTFILE" "$HASH" &&
    test $(stat -c "%Y" "$OUTFILE") -gt $NOW
  '

  # special case, because storing mtime requires dag-pb envelope
  # and when dealing with CIDv1 we can have 'raw' block instead of 'dag-pb'
  # so it needs to be converted to dag-pb before adding attribute
  test_expect_success "can add file modification time to cidv1 raw block [$1]" '
    NAME=$(mk_name) &&
    OUTFILE="$(mk_file "$NAME")" &&
    EXPECTED="$CUSTOM_MTIME" &&
    HASH=$(date | ipfs add -q --cid-version 1 --raw-leaves=true) &&
    ipfs files cp "/ipfs/$HASH" /$NAME &&
    ipfs files touch --mtime=$EXPECTED /$NAME &&
    test $(ipfs files stat --format="<mtime-secs>" "/$NAME") -eq $EXPECTED &&
    HASH=$(ipfs files stat /$NAME|head -1) &&
    ipfs get -o "$OUTFILE" "$HASH" &&
    test $(stat -c "%Y" "$OUTFILE") -eq $EXPECTED
  '

  test_expect_success "can change file modification time nanoseconds [$1]" '
    NAME=$(mk_name) &&
    echo test|ipfs files write --create /$NAME &&
    EXPECTED=$(date --date="yesterday" +%s) &&
    ipfs files touch --mtime=$EXPECTED --mtime-nsecs=55567 /$NAME &&
    test $(ipfs files stat --format="<mtime-secs>" /$NAME) -eq $EXPECTED &&
    test $(ipfs files stat --format="<mtime-nsecs>" /$NAME) -eq 55567
  '

  ## TODO: update these tests if/when symbolic links are fully supported in go-mfs
  test_expect_success "can change symlink modification time [$1]" '
    NAME=$(mk_name) &&
    EXPECTED=$(date +%s) &&
    ipfs files cp "/ipfs/$HASH_PRESERVE_LINK_MTIME" "/$NAME" ||
    ipfs files touch --mtime=$EXPECTED "/$NAME" &&
    test $(ipfs files stat --format="<mtime-secs>" "/$NAME") -eq $EXPECTED
  '

  test_expect_success "can change symlink modification time nanoseconds [$1]" '
    NAME=$(mk_name) &&
    EXPECTED=$(date +%s) &&
    ipfs files cp "/ipfs/$HASH_PRESERVE_LINK_MTIME" "/$NAME" ||
    ipfs files touch --mtime=$EXPECTED --mtime-nsecs=938475 "/$NAME" &&
    test $(ipfs files stat --format="<mtime-secs>" "/$NAME") -eq $EXPECTED &&
    test $(ipfs files stat --format="<mtime-nsecs>" "/$NAME") -eq 938475
  '
}

DIR_TIME=1655158632

setup_directory() {

  local TESTDIR="$(mktemp -d -p "$FIXTURESDIR" "${1}XXXXXX")"
  mkdir -p "$TESTDIR"/{dir1,dir2/sub1/sub2,dir3}
  chmod 0755 "$TESTDIR/dir1"

  touch -md @$(($DIR_TIME+10)) "$TESTDIR/dir2/sub1/sub2/file3"
  ln -s ../sub2/file3 "$TESTDIR/dir2/sub1/link1"
  touch -h -md @$(($DIR_TIME+20)) "$TESTDIR/dir2/sub1/link1"

  touch -md @$(($DIR_TIME+30)) "$TESTDIR/dir2/sub1/sub2"
  touch -md @$(($DIR_TIME+40)) "$TESTDIR/dir2/sub1"
  touch -md @$(($DIR_TIME+50)) "$TESTDIR/dir2"

  touch -md @$(($DIR_TIME+60)) "$TESTDIR/dir3/file2"
  touch -md @$(($DIR_TIME+70)) "$TESTDIR/dir3"

  touch -md @$(($DIR_TIME+80)) "$TESTDIR/file1"
  touch -md @$(($DIR_TIME+90)) "$TESTDIR/dir1"
  touch -md @$DIR_TIME "$TESTDIR"

  echo "$TESTDIR"
}

test_directory() {
  CUSTOM_DIR_MODE=0713
  TESTDIR=$(setup_directory $1)
  TESTDIR1="$TESTDIR/dir1"
  OUTDIR="$(mk_dir "${1}")"
  HASH_DIR_ROOT=QmSioyvQuXetxg7uo8FswGn9XKKEsisDq1HTMzGyWbw2R6
  HASH_DIR1_NO_PRESERVE=QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn
  HASH_DIR1_PRESERVE_MODE=QmRviohgafvCsbkiTgfQFipbuXJ6k1YtoiaQW4quttJPKu
  HASH_DIR1_PRESERVE_MTIME=QmYMy7CZGb498QFSQBF5ZFwv1FYbrAtYZMe4VxhDXxAcvf
  HASH_DIR1_CUSTOM_MODE=QmQ1ABnw2iip7sj23EzzBZ9T77KyyfESP6SUboiXPyzNQe
  HASH_DIR1_CUSTOM_MTIME=QmfWitW6F13WHFXLbJzXRYmwrS1p4gaAJAfucUSMytRPn3
  HASH_DIR1_CUSTOM_MTIME_NSECS=QmZFdCLJay31hT3Tx1LygJ7XfiLEs3qLCXtbeBfhf38aZg
  HASH_DIR_SUB1=QmeQwX5qAX18fcPDxDdkfM6ttuFCZetF5hgeUa6ov8D5oc

  HASH_DIR_MODE_AND_MTIME=(
    QmRCG3Pprg4jbhfYBzVzfJVyneFHnBquPGXwvXU3jSuf5j
    QmReHCn4BSJJdtd6Le8Hd8Puai6TmgpPCYb13wyM7FD9AD
    QmSioyvQuXetxg7uo8FswGn9XKKEsisDq1HTMzGyWbw2R6
    QmTMoVgJKhPrz9DfkvT132mxyBXNae5azXQ42WbM9abdSE
    QmVzXqpuQGCAgRwEbGuE9xe8Fidi1HEXaPKsQEFEbPJW9j
    QmW6Nqy2nziduAp3UGx2a52gtSUsYzhVcZMuPdxBRnwCyP
    QmeQwX5qAX18fcPDxDdkfM6ttuFCZetF5hgeUa6ov8D5oc
    QmefofUNwC2U3Xp87rB1x8Aws6AdsDuoXR7B9u2RkEZ4dQ
    Qmeu24TFarJwLzJgMTDYDJTr4BMGnzafoSnfxov1513abW
    Qmf82bbFg2e8HmcqiewutVVw5NoMpiXZD57LpLdC1poBuH)
  HASH_DIR_CUSTOM_MODE=(
    QmNZ5cyx3f6maXkczwhh3ufjDCh9f3k9zrDhX218ZZGvoV
    QmRqtFVLkXfWJuqWtYiCPthgomo3gouno8uvMeGAyCVaWS
    QmSkrWNcyDA7s1qiT6Ps7ey4zcB7uBH3sqGcKRfW4UMKhM
    QmSkrWNcyDA7s1qiT6Ps7ey4zcB7uBH3sqGcKRfW4UMKhM
    QmSkrWNcyDA7s1qiT6Ps7ey4zcB7uBH3sqGcKRfW4UMKhM
    QmZNAZXB6JyJ1cK9h1uJEK4XDo1CKsSuHMPGUUMrzDXCQz
    QmbSz6GyS8MNR4M9xtCteuGVJQRYkCXLbW174Fdy8jtaoZ
    QmccnAQQeJGtmtgZoi3hpEmgdxbuX1ao2hQmrKmmwQnCn9
    QmeTZoiAiduFY2hXaNQP4ehiE71BrQFEnrqduBZ5ZjHuFy
    Qmf13KNurvAHUfMBhMWvZuftmUikhhGY7ohWVaBDDndFMz)
  HASH_DIR_CUSTOM_MTIME=(
    QmPCGFZ8ZFowAwfWdCeGsr9wSbGXwZiHW3bZ7XSYcc1Zby
    QmT3aY4avDcYXCWpU8CJzqUkW7YEuEsx36S8cTNoLcuK1B
    QmT3aY4avDcYXCWpU8CJzqUkW7YEuEsx36S8cTNoLcuK1B
    QmT3aY4avDcYXCWpU8CJzqUkW7YEuEsx36S8cTNoLcuK1B
    QmUGMu9epCEz5HMsuJFgpJxxt3HoahsTQcC65Jje6LNqYF
    QmXhzoPKuqmkqbyr4kJFznFRXtGwriCXKGFPr4vviyK3aV
    QmZ5wKCcL11TckypuDTKLLNFP6JMCBJRCn385XKQQ6PCLt
    Qmdw3hiAxn6R5MRkkdzLdFvZUa2WJeLCTXXCyB8byFsHSA
    QmedF4m2Y8341azfkpvaHSkxbSrZa4fo6FT25h6sRUVkpq
    QmfWitW6F13WHFXLbJzXRYmwrS1p4gaAJAfucUSMytRPn3)

  test_expect_success "feature on directory has no effect when not used [$1]" '
    HASH=$(ipfs add -qr "$TESTDIR1") &&
    test "$HASH_DIR1_NO_PRESERVE" = "$HASH"
  '

  test_expect_success "can preserve directory mode [$1]" '
    HASH=$(ipfs add -qr --preserve-mode "$TESTDIR1") &&
    test "$HASH_DIR1_PRESERVE_MODE" = "$HASH"
  '

  test_expect_success "can preserve directory modification time [$1]" '
    HASH=$(ipfs add -qr --preserve-mtime "$TESTDIR1") &&
    test "$HASH_DIR1_PRESERVE_MTIME" = "$HASH"
  '

  test_expect_success "can set directory mode [$1]" '
    HASH=$(ipfs add -qr --mode=$CUSTOM_DIR_MODE "$TESTDIR1") &&
    test "$HASH_DIR1_CUSTOM_MODE" = "$HASH"
  '

  test_expect_success "can set directory modification time [$1]" '
    HASH=$(ipfs add -qr --mtime=$CUSTOM_MTIME "$TESTDIR1") &&
    test "$HASH_DIR1_CUSTOM_MTIME" = "$HASH"
  '

  test_expect_success "can set directory modification time nanoseconds [$1]" '
    HASH=$(ipfs add -qr --mtime=$CUSTOM_MTIME --mtime-nsecs=$CUSTOM_MTIME_NSECS "$TESTDIR1") &&
    test "$HASH_DIR1_CUSTOM_MTIME_NSECS" = "$HASH"
  '

  test_expect_success "can recursively preserve mode and modification time [$1]" '
    test "700:$DIR_TIME" = "$(stat -c "%a:%Y" "$TESTDIR")" &&
    test "644:$((DIR_TIME+10))" = "$(stat -c "%a:%Y" "$TESTDIR/dir2/sub1/sub2/file3")" &&
    test "777:$((DIR_TIME+20))" = "$(stat -c "%a:%Y" "$TESTDIR/dir2/sub1/link1")" &&
    test "755:$((DIR_TIME+30))" = "$(stat -c "%a:%Y" "$TESTDIR/dir2/sub1/sub2")" &&
    test "755:$((DIR_TIME+40))" = "$(stat -c "%a:%Y" "$TESTDIR/dir2/sub1")" &&
    test "755:$((DIR_TIME+50))" = "$(stat -c "%a:%Y" "$TESTDIR/dir2")" &&
    test "644:$((DIR_TIME+60))" = "$(stat -c "%a:%Y" "$TESTDIR/dir3/file2")" &&
    test "755:$((DIR_TIME+70))" = "$(stat -c "%a:%Y" "$TESTDIR/dir3")" &&
    test "644:$((DIR_TIME+80))" = "$(stat -c "%a:%Y" "$TESTDIR/file1")" &&
    test "755:$((DIR_TIME+90))" = "$(stat -c "%a:%Y" "$TESTDIR/dir1")" &&
    HASHES=($(ipfs add -qr --preserve-mode --preserve-mtime "$TESTDIR"|sort)) &&
    test "${HASHES[*]}" = "${HASH_DIR_MODE_AND_MTIME[*]}"
  '

  test_expect_success "can recursively set directory mode [$1]" '
    HASHES=($(ipfs add -qr --mode=0753 "$TESTDIR"|sort)) &&
    test "${HASHES[*]}" = "${HASH_DIR_CUSTOM_MODE[*]}"
  '

  test_expect_success "can recursively set directory mtime [$1]" '
    HASHES=($(ipfs add -qr --mtime=$CUSTOM_MTIME "$TESTDIR"|sort)) &&
    test "${HASHES[*]}" = "${HASH_DIR_CUSTOM_MTIME[*]}"
  '

  test_expect_success "can recursively restore mode and mtime [$1]" '
    ipfs get -o "$OUTDIR" $HASH_DIR_ROOT &&
    test "700:$DIR_TIME" = "$(stat -c "%a:%Y" "$OUTDIR")" &&
    test "644:$((DIR_TIME+10))" = "$(stat -c "%a:%Y" "$OUTDIR/dir2/sub1/sub2/file3")" &&
    test "777:$((DIR_TIME+20))" = "$(stat -c "%a:%Y" "$OUTDIR/dir2/sub1/link1")" &&
    test "755:$((DIR_TIME+30))" = "$(stat -c "%a:%Y" "$OUTDIR/dir2/sub1/sub2")" &&
    test "755:$((DIR_TIME+40))" = "$(stat -c "%a:%Y" "$OUTDIR/dir2/sub1")" &&
    test "755:$((DIR_TIME+50))" = "$(stat -c "%a:%Y" "$OUTDIR/dir2")" &&
    test "644:$((DIR_TIME+60))" = "$(stat -c "%a:%Y" "$OUTDIR/dir3/file2")" &&
    test "755:$((DIR_TIME+70))" = "$(stat -c "%a:%Y" "$OUTDIR/dir3")" &&
    test "644:$((DIR_TIME+80))" = "$(stat -c "%a:%Y" "$OUTDIR/file1")" &&
    test "755:$((DIR_TIME+90))" = "$(stat -c "%a:%Y" "$OUTDIR/dir1")"
  '

  # basic smoke-test for cidv1 (we dont care about CID, just care about
  # mode/mtime surviving ipfs import and export if --cid-version 1 is at play)
  test_expect_success "can recursively preserve and restore mode and mtime with CIDv1 [$1]" '
    test "700:$DIR_TIME" = "$(stat -c "%a:%Y" "$TESTDIR")" &&
    test "644:$((DIR_TIME+10))" = "$(stat -c "%a:%Y" "$TESTDIR/dir2/sub1/sub2/file3")" &&
    test "777:$((DIR_TIME+20))" = "$(stat -c "%a:%Y" "$TESTDIR/dir2/sub1/link1")" &&
    test "755:$((DIR_TIME+30))" = "$(stat -c "%a:%Y" "$TESTDIR/dir2/sub1/sub2")" &&
    test "755:$((DIR_TIME+40))" = "$(stat -c "%a:%Y" "$TESTDIR/dir2/sub1")" &&
    test "755:$((DIR_TIME+50))" = "$(stat -c "%a:%Y" "$TESTDIR/dir2")" &&
    test "644:$((DIR_TIME+60))" = "$(stat -c "%a:%Y" "$TESTDIR/dir3/file2")" &&
    test "755:$((DIR_TIME+70))" = "$(stat -c "%a:%Y" "$TESTDIR/dir3")" &&
    test "644:$((DIR_TIME+80))" = "$(stat -c "%a:%Y" "$TESTDIR/file1")" &&
    test "755:$((DIR_TIME+90))" = "$(stat -c "%a:%Y" "$TESTDIR/dir1")" &&
    CIDV1DIR=$(ipfs add -Qr --preserve-mode --preserve-mtime --cid-version 1 "$TESTDIR") &&
    OUTDIRV1=$(mk_dir cidv1roundtrip$1) &&
    ipfs get -o "$OUTDIRV1" $CIDV1DIR &&
    test "700:$DIR_TIME" = "$(stat -c "%a:%Y" "$OUTDIRV1")" &&
    test "644:$((DIR_TIME+10))" = "$(stat -c "%a:%Y" "$OUTDIRV1/dir2/sub1/sub2/file3")" &&
    test "777:$((DIR_TIME+20))" = "$(stat -c "%a:%Y" "$OUTDIRV1/dir2/sub1/link1")" &&
    test "755:$((DIR_TIME+30))" = "$(stat -c "%a:%Y" "$OUTDIRV1/dir2/sub1/sub2")" &&
    test "755:$((DIR_TIME+40))" = "$(stat -c "%a:%Y" "$OUTDIRV1/dir2/sub1")" &&
    test "755:$((DIR_TIME+50))" = "$(stat -c "%a:%Y" "$OUTDIRV1/dir2")" &&
    test "644:$((DIR_TIME+60))" = "$(stat -c "%a:%Y" "$OUTDIRV1/dir3/file2")" &&
    test "755:$((DIR_TIME+70))" = "$(stat -c "%a:%Y" "$OUTDIRV1/dir3")" &&
    test "644:$((DIR_TIME+80))" = "$(stat -c "%a:%Y" "$OUTDIRV1/file1")" &&
    test "755:$((DIR_TIME+90))" = "$(stat -c "%a:%Y" "$OUTDIRV1/dir1")"
  '

  test_expect_success "can change directory mode [$1]" '
    NAME=$(mk_name) &&
    ipfs files cp "/ipfs/$HASH_DIR_SUB1" /$NAME &&
    ipfs files chmod 0710 /$NAME &&
    test $(ipfs files stat --format="<mode>" /$NAME) = "drwx--x---"
  '

  test_expect_success "can change directory modification time [$1]" '
    NAME=$(mk_name) &&
    ipfs files cp "/ipfs/$HASH_DIR_SUB1" /$NAME &&
    ipfs files touch --mtime=$CUSTOM_MTIME /$NAME &&
    test $(ipfs files stat --format="<mtime-secs>" /$NAME) -eq $CUSTOM_MTIME
  '

  test_expect_success "can change directory modification time nanoseconds [$1]" '
    NAME=$(mk_name) &&
    MTIME=$(date --date="yesterday" +%s) &&
    ipfs files cp "/ipfs/$HASH_DIR_SUB1" /$NAME &&
    ipfs files touch --mtime=$MTIME --mtime-nsecs=94783 /$NAME &&
    test $(ipfs files stat --format="<mtime-secs>" /$NAME) -eq $MTIME &&
    test $(ipfs files stat --format="<mtime-nsecs>" /$NAME) -eq 94783
  '
}

test_stat_template() {
  test_expect_success "can stat $2 string mode [$1]" '
    touch "$STAT_TARGET" &&
    HASH=$(ipfs add -qr --mode="$STAT_MODE_OCTAL" "$STAT_TARGET") &&
    ACTUAL=$(ipfs files stat --format="<mode>" /ipfs/$HASH) &&
    test "$ACTUAL" = "$STAT_MODE_STRING"
  '
  test_expect_success "can stat $2 octal mode [$1]" '
    touch "$STAT_TARGET" &&
    HASH=$(ipfs add -qr --mode="$STAT_MODE_OCTAL" "$STAT_TARGET") &&
    ACTUAL=$(ipfs files stat --format="<mode-octal>" /ipfs/$HASH) &&
    test "$ACTUAL" = "$STAT_MODE_OCTAL"
  '

  test_expect_success "can stat $2 modification time string [$1]" '
    touch "$STAT_TARGET" &&
    HASH=$(ipfs add -qr --mtime=$CUSTOM_MTIME "$STAT_TARGET") &&
    ACTUAL=$(ipfs files stat --format="<mtime>" /ipfs/$HASH) &&
    test "$ACTUAL" = "24 Oct 2020, 11:42:00 UTC"
  '

  test_expect_success "can stat $2 modification time seconds [$1]" '
    touch "$STAT_TARGET" &&
    HASH=$(ipfs add -qr --mtime=$CUSTOM_MTIME "$STAT_TARGET") &&
    ACTUAL=$(ipfs files stat --format="<mtime-secs>" /ipfs/$HASH) &&
    test $ACTUAL -eq $CUSTOM_MTIME
  '

  test_expect_success "can stat $2 modification time nanoseconds [$1]" '
    touch "$STAT_TARGET" &&
    HASH=$(ipfs add -qr --mtime=$CUSTOM_MTIME --mtime-nsecs=$CUSTOM_MTIME_NSECS "$STAT_TARGET") &&
    ACTUAL=$(ipfs files stat --format="<mtime-nsecs>" /ipfs/$HASH) &&
    test $ACTUAL -eq $CUSTOM_MTIME_NSECS
  '
}

test_stat() {
  STAT_TARGET="$FIXTURESDIR/statfile$1"
  STAT_MODE_OCTAL="$CUSTOM_MODE"
  STAT_MODE_STRING="-rwxrw-r--"
  test_stat_template "$1" "file"

  STAT_TARGET="$FIXTURESDIR/statdir$1"
  STAT_MODE_OCTAL="0731"
  STAT_MODE_STRING="drwx-wx--x"
  mkdir "$STAT_TARGET"
  test_stat_template "$1" "directory"

  STAT_TARGET="$FIXTURESDIR/statlink$1"
  STAT_MODE_OCTAL="0777"
  STAT_MODE_STRING="lrwxrwxrwx"
  ln -s nothing "$STAT_TARGET"
  test_stat_template "$1" "link"


  STAT_TARGET="$FIXTURESDIR/statfile$1"
  test_expect_success "can chain stat template [$1]" '
    HASH=$(ipfs add -q --mode=0644 --mtime=$CUSTOM_MTIME --mtime-nsecs=$CUSTOM_MTIME_NSECS "$STAT_TARGET") &&
    ACTUAL=$(ipfs files stat --format="<mtime> <mtime-secs> <mtime-nsecs> <mode> <mode-octal>" /ipfs/$HASH) &&
    test "$ACTUAL" = "24 Oct 2020, 11:42:00 UTC 1603539720 54321 -rw-r--r-- 0644"
  '
}

test_all() {
test_stat "$1"
test_file "$1"
test_directory "$1"
}

# test direct
test_all "direct"

# test daemon
test_launch_ipfs_daemon_without_network
test_all "daemon"
test_kill_ipfs_daemon

test_done
