#!/usr/bin/env bash
#
# Copyright (c) 2015 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="test dns resolution of api endpoint by cli"

. lib/test-lib.sh

test_init_ipfs

# this test uses the localtest.me domain which resolves to 127.0.0.1
# see http://readme.localtest.me/
# in case if failure, check A record of that domain
test_expect_success "can make http request against dns resolved nc server" '
  nc -ld 5005 > nc_out &
  NCPID=$!
  go-sleep 0.5s && kill "$NCPID" &
  ipfs cat /ipfs/Qmabcdef --api /dns4/localtest.me/tcp/5005 || true
'

test_expect_success "request was received by local nc server" '
  grep "POST /api/v0/cat" nc_out
'

test_done
