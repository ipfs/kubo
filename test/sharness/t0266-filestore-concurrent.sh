#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test filestore"

. lib/test-filestore-lib.sh
. lib/test-lib.sh

test_init_ipfs

test_enable_filestore

export IPFS_LOGGING_FMT=nocolor

test_launch_ipfs_daemon --offline

test_expect_success "enable filestore debug logging" '
  ipfs log level filestore debug
'

test_expect_success "generate 500MB file using go-random" '
    random 524288000 41 >mountdir/hugefile
'

test_expect_success "'filestore clean orphan race condition" '(
    set -m
    (ipfs filestore add -q --logical mountdir/hugefile > hugefile-hash && echo "add done") &
    sleep 1 &&
    (ipfs filestore clean orphan && echo "clean done") &
    wait
)'

test_kill_ipfs_daemon

cat <<EOF > filtered_expect
acquired add-lock refcnt now 1
Starting clean operation.
Removing invalid blocks after clean.  Online Mode.
released add-lock refcnt now 0
EOF

test_expect_success "filestore clean orphan race condition: operations ran in correct order" '
  egrep -i "add-lock|clean" daemon_err | cut -d " " -f 6- > filtered_actual &&
  test_cmp filtered_expect filtered_actual
'

test_expect_success "filestore clean orphan race condition: file still accessible" '
   ipfs cat `cat hugefile-hash` > /dev/null
'

export IPFS_FILESTORE_CLEAN_RM_DELAY=5s

test_launch_ipfs_daemon --offline

test_expect_success "ipfs add succeeds" '
    echo "Hello Worlds!" >mountdir/hello.txt &&
    ipfs filestore add --logical mountdir/hello.txt >actual &&
    HASH="QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH" &&
    ipfs cat "$HASH" > /dev/null
'

test_expect_success "fail after file move" '
    mv mountdir/hello.txt mountdir/hello2.txt
    test_must_fail ipfs cat "$HASH" >/dev/null
'

test_expect_success "filestore clean invalid race condation" '(
  set -m
  ipfs filestore clean invalid > clean-actual &
  sleep 2 &&
  ipfs filestore add --logical mountdir/hello2.txt &&
  wait
)'

test_expect_success "filestore clean race condation: output looks good" '
  grep "cannot remove $HASH" clean-actual
'

test_expect_success "filestore clean race condation: file still available" '
  ipfs cat "$HASH" > /dev/null
'

test_kill_ipfs_daemon

unset IPFS_FILESTORE_CLEAN_RM_DELAY
export IPFS_FILESTORE_CLEAN_RM_DELAY

test_expect_success "fail after file move" '
    rm mountdir/hugefile
    test_must_fail ipfs cat `echo hugefile-hash` >/dev/null
'

export IPFS_LOGGING=debug

# note: exclusive mode deletes do not check if a DataObj has changed
# from under us and are thus likley to be faster when cleaning out
# a large number of invalid blocks
test_expect_success "ipfs clean local mode uses exclusive mode" '
  ipfs filestore clean invalid > clean-out 2> clean-err &&
  grep "Exclusive Mode." clean-err
'

test_done
