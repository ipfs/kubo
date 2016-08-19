#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test filestore"

. lib/test-lib.sh

test_init_ipfs

# add block
# add filestore block / rm file
# make sure gc still words

FILE1=QmfM2r8seH2GiRaC4esTjeraXEachRt8ZsSeGaWTPLyMoG
test_expect_success "add a pinned file" '
  echo "Hello World!" > file1 &&
  ipfs add file1
  ipfs cat $FILE1 | cmp file1
'

FILE2=QmPrrHqJzto9m7SyiRzarwkqPcCSsKR2EB1AyqJfe8L8tN
test_expect_success "add an unpinned file" '
  echo "Hello Mars!"  > file2
  ipfs add --pin=false file2
  ipfs cat $FILE2 | cmp file2
'

FILE3=QmeV1kwh3333bsnT6YRfdCRrSgUPngKmAhhTa4RrqYPbKT
test_expect_success "add and pin a directory using the filestore" '
  mkdir adir &&
  echo "hello world!" > adir/file3 && 
  echo "hello mars!"  > adir/file4 &&
  ipfs filestore add --logical -r --pin adir &&
  ipfs cat $FILE3 | cmp adir/file3
'

FILE5=QmU5kp3BH3B8tnWUU2Pikdb2maksBNkb92FHRr56hyghh4
test_expect_success "add a unpinned file to the filestore" '
   echo "Hello Venus!" > file5 &&
   ipfs filestore add --logical --pin=false file5 &&
   ipfs cat $FILE5 | cmp file5
'

test_expect_success "make sure filestore block is really not pinned" '
  test_must_fail ipfs pin ls $FILE5
'

test_expect_success "remove one of the backing files" '
  rm adir/file3 &&
  test_must_fail ipfs cat $FILE3
'

test_expect_success "make ipfs pin ls is still okay" '
  ipfs pin ls
'

test_expect_success "make sure the gc will still run" '
  ipfs repo gc
'

test_expect_success "make sure pinned block got removed after gc" '
   ipfs cat $FILE1
'

test_expect_success "make sure un-pinned block still exists" '
   test_must_fail ipfs cat $FILE2
'

test_expect_success "make sure unpinned filestore block did not get removed" '
  ipfs cat $FILE5
'

test_done
