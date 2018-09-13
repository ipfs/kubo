#!/usr/bin/env bash
#
# Copyright (c) 2018 Protocol Labs Inc.
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test API on Gateway"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

port=$GWAY_PORT

for cmd in add  \
           block/put \
           bootstrap \
           config \
           dht \
           diag \
           id \
           mount \
           name/publish \
           object/put \
           object/new \
           object/patch \
           pin \
           ping \
           repo \
           stats \
           swarm \
           file \
           update \
           bitswap
do
  test_expect_success "test gateway api is sanitized: $cmd" '
    test_curl_resp_http_code "http://127.0.0.1:$port/api/v0/$cmd" "HTTP/1.1 404 Not Found"
  '
done

# This one is different. `local` will be interpreted as a path if the command isn't defined.
test_expect_success "test gateway api is sanitized: refs/local" '
    echo "Error: invalid '"'ipfs ref'"' path" > refs_local_expected &&
    ! ipfs --api /ip4/127.0.0.1/tcp/$port refs local > refs_local_actual 2>&1 &&
    test_cmp refs_local_expected refs_local_actual
  '

test_expect_success "GET IPFS directory path succeeds" '
  mkdir dir &&
  echo "12345" >dir/test &&
  ipfs add -r -q dir >actual &&
  HASH2=$(tail -n 1 actual)
'

test_curl_gateway_api() {
  curl -sfo actual "http://127.0.0.1:$port/api/v0/$1"
}

test_expect_success "get IPFS directory file through readonly API succeeds" '
  test_curl_gateway_api "cat?arg=$HASH2/test"
'

test_expect_success "get IPFS directory file through readonly API output looks good" '
  test_cmp dir/test actual
'

test_expect_success "refs IPFS directory file through readonly API succeeds" '
  test_curl_gateway_api "refs?arg=$HASH2/test"
'

test_kill_ipfs_daemon
test_done
