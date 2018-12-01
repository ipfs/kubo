#!/usr/bin/env bash
#
# Copyright (c) 2015 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="test dns resolution of api endpoint by cli"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "start nc" '
  rm -f nc_out nc_outp nc_inp && mkfifo nc_inp nc_outp

  nc -k -l 127.0.0.1 5006 <nc_inp >nc_outp &
  NCPID=$!

  exec 6>nc_inp 7<nc_outp

  while ! nc -z 127.0.0.1 5006; do
    go-sleep 100ms
  done
'

test_expect_success "can make http request against dns resolved nc server" '
  ipfs cat /ipfs/Qmabcdef --api /dns4/localhost/tcp/5006 &
  IPFSPID=$!

  # handle request
  while read line; do
    if [[ "$line" == "$(echo -e "\r")" ]]; then
      break
    fi
    echo "$line"
  done <&7 >nc_out &&

  echo -e "HTTP/1.1 200 OK\r" >&6 &&
  echo -e "Content-Type: text/plain\r" >&6 &&
  echo -e "Content-Length: 0\r" >&6 &&
  echo -e "\r" >&6 &&
  exec 6<&- &&

  # Wait for IPFS
  wait $IPFSPID
'

test_expect_success "stop nc" '
  kill "$NCPID" && wait "$NCPID" || true
'

test_expect_success "request was received by local nc server" '
  grep "POST /api/v0/cat" nc_out
'

test_done
