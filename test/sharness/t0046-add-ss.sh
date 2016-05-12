#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test filestore"

. lib/test-lib.sh

client_err() {
    printf "$@\n\nUse 'ipfs add --help' for information about this command\n"
}

test_init_ipfs

test_launch_ipfs_daemon

test_expect_success "ipfs add-ss fails unless enable" '
  echo "Hello Worlds!" >mountdir/hello.txt &&
  test_must_fail ipfs add-ss "`pwd`"/mountdir/hello.txt >actual
'

test_kill_ipfs_daemon

test_expect_success "enable API.ServerSideAdds" '
  ipfs config API.ServerSideAdds --bool true
'

test_launch_ipfs_daemon

test_expect_success "ipfs add-ss okay after enabling" '
  echo "Hello Worlds!" >mountdir/hello.txt &&
  ipfs add-ss "`pwd`"/mountdir/hello.txt >actual
'

test_kill_ipfs_daemon

test_done
