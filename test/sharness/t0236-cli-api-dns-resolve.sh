#!/usr/bin/env bash
#
# Copyright (c) 2015 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="test dns resolution of api endpoint by cli"

. lib/test-lib.sh

if ! test_have_prereq SOCAT; then
  skip_all="skipping '$test_description': socat is not available"
  test_done
fi

test_init_ipfs

test_expect_success "start nc" '
  rm -f nc_out nc_outp nc_inp && mkfifo nc_inp nc_outp

  socat PIPE:nc_inp!!PIPE:nc_outp tcp-listen:5006,fork,max-children=1,bind=127.0.0.1 &
  NCPID=$!

  exec 6>nc_inp 7<nc_outp

  socat /dev/null tcp:127.0.01:5006,retry=10
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
