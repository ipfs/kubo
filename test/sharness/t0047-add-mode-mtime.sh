#!/usr/bin/env bash

test_description="Test storing and retrieving mode and mtime"

. lib/test-lib.sh

test_init_ipfs

HASH_NO_PRESERVE=QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH

PRESERVE_MTIME=1604320482
PRESERVE_MODE=0640
HASH_PRESERVE_MODE=QmQLgxypSNGNFTuUPGCecq6dDEjb6hNB5xSyVmP3cEuNtq
HASH_PRESERVE_MTIME=QmQ6kErEW8kztQFV8vbwNU8E4dmtGsYpRiboiLxUEwibvj
HASH_PRESERVE_MODE_AND_MTIME=QmYkvboLsvLFcSYmqVJRxvBdYRQLroLv9kELf3LRiCqBri

CUSTOM_MTIME=1603539720
CUSTOM_MTIME_NSECS=54321
CUSTOM_MODE=0764
HASH_CUSTOM_MODE=QmchD3BN8TQ3RW6jPLxSaNkqvfuj7syKhzTRmL4EpyY1Nz
HASH_CUSTOM_MTIME=QmT3aY4avDcYXCWpU8CJzqUkW7YEuEsx36S8cTNoLcuK1B
HASH_CUSTOM_MTIME_NSECS=QmaKH8H5rXBUBCX4vdxi7ktGQEL7wejV7L9rX2qpZjwncz
HASH_CUSTOM_MODE_AND_MTIME=QmUkxrtBA8tPjwCYz1HrsoRfDz6NgKut3asVeHVQNH4C8L

mk_name() {
  tr -dc '[:alnum:]'</dev/urandom|head -c 16
}

test_file() {
  TESTFILE=mountdir/test$1.txt

  test_expect_success "feature has no effect when not used [$1]" '
    touch -d @$PRESERVE_MTIME "$TESTFILE" &&
    HASH=$(ipfs add -q --hash=sha2-256 "$TESTFILE") &&
    test "$HASH_NO_PRESERVE" = "$HASH"    
  '

  test_expect_success "can preserve file mode [$1]" '
    touch "$TESTFILE" &&
    chmod $PRESERVE_MODE "$TESTFILE" &&
    HASH=$(ipfs add -q --hash=sha2-256 --preserve-mode "$TESTFILE") &&
    test "$HASH_PRESERVE_MODE" = "$HASH"
  '

  test_expect_success "can preserve file modification time [$1]" '
    touch -m -d @$PRESERVE_MTIME "$TESTFILE" &&
    HASH=$(ipfs add -q --hash=sha2-256 --preserve-mtime "$TESTFILE") &&
    test "$HASH_PRESERVE_MTIME" = "$HASH"
  '

  test_expect_success "can set file mode [$1]" '
    touch "$TESTFILE" &&
    chmod 0600 "$TESTFILE" &&
    HASH=$(ipfs add -q --hash=sha2-256 --mode=$CUSTOM_MODE "$TESTFILE") &&
    test "$HASH_CUSTOM_MODE" = "$HASH"
  '

  test_expect_success "can set file mtime [$1]" '
    touch -m -t 202011021234.42 "$TESTFILE" &&
    HASH=$(ipfs add -q --hash=sha2-256 --mtime=$CUSTOM_MTIME "$TESTFILE") &&
    test "$HASH_CUSTOM_MTIME" = "$HASH"
  '

  test_expect_success "can set file mtime nanoseconds [$1]" '
    touch -m -t 202011021234.42 "$TESTFILE" &&
    HASH=$(ipfs add -q --hash=sha2-256 --mtime=$CUSTOM_MTIME --mtime-nsecs=$CUSTOM_MTIME_NSECS "$TESTFILE") &&
    test "$HASH_CUSTOM_MTIME_NSECS" = "$HASH"
  '

  test_expect_success "can preserve file mode and modification time [$1]" '
    touch -m -d @$PRESERVE_MTIME "$TESTFILE" &&
    chmod $PRESERVE_MODE "$TESTFILE" &&
    HASH=$(ipfs add -q --hash=sha2-256 --preserve-mode --preserve-mtime "$TESTFILE") &&
    test "$HASH_PRESERVE_MODE_AND_MTIME" = "$HASH"
  '

  test_expect_success "can set file mode and mtime [$1]" '
    touch -m -t 202011021234.42 "$TESTFILE" &&
    chmod 0600 "$TESTFILE" &&
    HASH=$(ipfs add -q --hash=sha2-256 --mode=$CUSTOM_MODE --mtime=$CUSTOM_MTIME --mtime-nsecs=$CUSTOM_MTIME_NSECS "$TESTFILE") &&
    test "$HASH_CUSTOM_MODE_AND_MTIME" = "$HASH"
  '

  test_expect_success "can get preserved mode and mtime [$1]" '
    OUTFILE="mountdir/$HASH_PRESERVE_MODE_AND_MTIME" &&
    ipfs get -o "$OUTFILE" $HASH_PRESERVE_MODE_AND_MTIME &&
    test "$PRESERVE_MODE:$PRESERVE_MTIME" = "$(stat -c "0%a:%Y" "$OUTFILE")"
  '

  test_expect_success "can get custom mode and mtime [$1]" '
    OUTFILE="mountdir/$HASH_CUSTOM_MODE_AND_MTIME" &&
    ipfs get -o "$OUTFILE" $HASH_CUSTOM_MODE_AND_MTIME &&
    TIMESTAMP=$(date +%s%N --date="$(stat -c "%y" $OUTFILE)") &&
    MODETIME=$(stat -c "0%a:$TIMESTAMP" "$OUTFILE") &&
    printf -v EXPECTED "$CUSTOM_MODE:$CUSTOM_MTIME%09d" $CUSTOM_MTIME_NSECS &&
    test "$EXPECTED" = "$MODETIME"
  '

  test_expect_success "can change file mode [$1]" '
    NAME=$(mk_name) &&
    HASH=$(echo testfile | ipfs add -q --mode=0600) &&
    ipfs files cp "/ipfs/$HASH" /$NAME &&
    ipfs files chmod 444 /$NAME &&
    HASH=$(ipfs files stat /$NAME|head -1) &&
    ipfs get -o mountdir/$NAME $HASH &&
    test $(stat -c "%a" mountdir/$NAME) = 444
  '

  test_expect_success "can touch file modification time [$1]" '
    NAME=$(mk_name) &&
    NOW=$(date +%s) &&
    HASH=$(echo testfile | ipfs add -q --mtime=$NOW) &&
    ipfs files cp "/ipfs/$HASH" /$NAME &&
    sleep 1 &&
    ipfs files touch /$NAME &&
    HASH=$(ipfs files stat /$NAME|head -1) &&
    ipfs get -o mountdir/$NAME $HASH &&
    test $(stat -c "%Y" mountdir/$NAME) -gt $NOW
  '

  test_expect_success "can touch file modification time and nanoseconds [$1]" '
    NAME=$(mk_name) &&
    echo test|ipfs files write --create /$NAME &&
    EXPECTED=$(date --date="yesterday" +%s) &&
    ipfs files touch --mtime=$EXPECTED --mtime-nsecs=55567 /$NAME &&
    test $(ipfs files stat --format="<mtime-secs>" /$NAME) -eq $EXPECTED &&
    test $(ipfs files stat --format="<mtime-nsecs>" /$NAME) -eq 55567
  '
}

TIME=1655158632

setup_directory() {
  local TESTDIR=$(mktemp -d -p mountdir "${1}XXXXXX")
  mkdir -p "$TESTDIR"/{dir1,dir2/sub1/sub2,dir3}

  touch -md @$(($TIME+10)) $TESTDIR/dir2/sub1/sub2/file3
  ln -s ../sub2/file3 $TESTDIR/dir2/sub1/link1
  touch -h -md @$(($TIME+20)) $TESTDIR/dir2/sub1/link1
  
  touch -md @$(($TIME+30)) $TESTDIR/dir2/sub1/sub2
  touch -md @$(($TIME+40)) $TESTDIR/dir2/sub1
  touch -md @$(($TIME+50)) $TESTDIR/dir2
  
  touch -md @$(($TIME+60)) $TESTDIR/dir3/file2
  touch -md @$(($TIME+70)) $TESTDIR/dir3

  touch -md @$(($TIME+80)) $TESTDIR/file1  
  touch -md @$(($TIME+90)) $TESTDIR/dir1
  touch -md @$TIME $TESTDIR

  echo "$TESTDIR"
}

test_directory() {
  TESTDIR=$(setup_directory $1)
  OUTDIR="$(mktemp -d -p mountdir "out_${1}XXXXXX")"
  HASH_DIR_ROOT=QmSioyvQuXetxg7uo8FswGn9XKKEsisDq1HTMzGyWbw2R6
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
  
  test_expect_success "can preserve mode and mtime recursively [$1]" '
    HASHES=($(ipfs add -qr --preserve-mode --preserve-mtime "$TESTDIR"|sort)) &&
    test "${HASHES[*]}" = "${HASH_DIR_MODE_AND_MTIME[*]}"
  '

  test_expect_success "can recursively restore mode and mtime [$1]" '
    ipfs get -o "$OUTDIR" $HASH_DIR_ROOT &&
    test "700:$TIME" = "$(stat -c "%a:%Y" "$OUTDIR")" &&
    test "644:$((TIME+10))" = "$(stat -c "%a:%Y" "$OUTDIR/dir2/sub1/sub2/file3")" &&
    test "777:$((TIME+20))" = "$(stat -c "%a:%Y" "$OUTDIR/dir2/sub1/link1")" &&
    test "755:$((TIME+30))" = "$(stat -c "%a:%Y" "$OUTDIR/dir2/sub1/sub2")" &&
    test "755:$((TIME+40))" = "$(stat -c "%a:%Y" "$OUTDIR/dir2/sub1")" &&
    test "755:$((TIME+50))" = "$(stat -c "%a:%Y" "$OUTDIR/dir2")" &&
    test "644:$((TIME+60))" = "$(stat -c "%a:%Y" "$OUTDIR/dir3/file2")" &&
    test "755:$((TIME+70))" = "$(stat -c "%a:%Y" "$OUTDIR/dir3")" &&
    test "644:$((TIME+80))" = "$(stat -c "%a:%Y" "$OUTDIR/file1")" &&
    test "755:$((TIME+90))" = "$(stat -c "%a:%Y" "$OUTDIR/dir1")"
  '
}

# test direct
test_file "direct"
test_directory "direct"

# test via daemon
test_launch_ipfs_daemon_without_network
test_file "daemon"
test_directory "daemon"
test_kill_ipfs_daemon

test_done
