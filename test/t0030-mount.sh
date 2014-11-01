#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test mount command"

. ./test-lib.sh

test_launch_ipfs_mount

test_kill_ipfs_mount

test_done
