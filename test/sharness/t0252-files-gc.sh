#!/bin/sh
#
# Copyright (c) 2016 Jeromy Johnson
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

test_expect_success "gc okay after adding incomplete node -- prep" '
  ipfs files mkdir /adir &&
  echo "file1" |  ipfs files write --create /adir/file1 &&
  echo "file2" |  ipfs files write --create /adir/file2 &&
  ipfs pin add --recursive=false QmbCgoMYVuZq8m1vK31JQx9DorwQdLMF1M3sJ7kygLLqnW &&
  ipfs files rm -r /adir &&
  ipfs repo gc && # will remove /adir/file1 and /adir/file2 but not /adir
  ipfs files cp /ipfs/QmbCgoMYVuZq8m1vK31JQx9DorwQdLMF1M3sJ7kygLLqnW /adir &&
  ipfs pin rm QmbCgoMYVuZq8m1vK31JQx9DorwQdLMF1M3sJ7kygLLqnW
'

test_expect_success "gc okay after adding incomplete node" '
  ipfs refs QmbCgoMYVuZq8m1vK31JQx9DorwQdLMF1M3sJ7kygLLqnW &&
  ipfs repo gc &&
  ipfs refs QmbCgoMYVuZq8m1vK31JQx9DorwQdLMF1M3sJ7kygLLqnW
'

test_done
