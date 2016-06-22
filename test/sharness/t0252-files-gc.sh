#!/bin/sh
#
# Copyright (c) 2016 Kevin Atkinson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="test how the unix files api interacts with the gc"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "object not removed after gc" '
  echo "hello world"  | ipfs files write --create /hello.txt &&
  ipfs repo gc &&
  ipfs cat QmVib14uvPnCP73XaCDpwugRuwfTsVbGyWbatHAmLSdZUS
'

test_expect_success "/hello.txt still accessible after gc" '
  ipfs files read /hello.txt
'

ADIR_HASH=QmbCgoMYVuZq8m1vK31JQx9DorwQdLMF1M3sJ7kygLLqnW
FILE1_HASH=QmX4eaSJz39mNhdu5ACUwTDpyA6y24HmrQNnAape6u3buS

test_expect_success "gc okay after adding incomplete node -- prep" '
  ipfs files mkdir /adir &&
  echo "file1" |  ipfs files write --create /adir/file1 &&
  echo "file2" |  ipfs files write --create /adir/file2 &&
  ipfs cat $FILE1_HASH && 
  ipfs pin add --recursive=false $ADIR_HASH &&
  ipfs files rm -r /adir &&
  ipfs repo gc && # will remove /adir/file1 and /adir/file2 but not /adir
  test_must_fail ipfs cat $FILE1_HASH &&
  ipfs files cp /ipfs/$ADIR_HASH /adir &&
  ipfs pin rm $ADIR_HASH
'

test_expect_success "gc okay after adding incomplete node" '
  ipfs refs $ADIR_HASH &&
  ipfs repo gc &&
  ipfs refs $ADIR_HASH
'

test_done
