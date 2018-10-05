#!/usr/bin/env bash
#
# Copyright (c) 2018 Protocol Labs Inc.
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test API on Gateway"

. lib/test-lib.sh

test_init_ipfs

test_curl_gateway_api() {
  curl -sfo "$1" "http://127.0.0.1:$port/api/v0/$2"
}

test_expect_success 'allow API command' '
  ipfs config Gateway.APICommands --json '"'"'["config"]'"'"'
'

test_launch_ipfs_daemon
port=$GWAY_PORT

test_expect_success 'verify allowed API command' '
  echo '"'"'{"Key":"Gateway.APICommands","Value":["config"]}'"'"' >> commands_expected &&
  test_curl_gateway_api commands_actual "config?arg=Gateway.APICommands" &&
  test_cmp commands_expected commands_actual
'

test_expect_success 'revert to default API commands' '
  ipfs config Gateway.APICommands --json '"'"'[]'"'"'
'

test_kill_ipfs_daemon
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

test_expect_success "get IPFS directory file through readonly API succeeds" '
  test_curl_gateway_api dir_actual "cat?arg=$HASH2/test"
'

test_expect_success "get IPFS directory file through readonly API output looks good" '
  test_cmp dir/test dir_actual
'

test_expect_success "refs IPFS directory file through readonly API succeeds" '
  test_curl_gateway_api file_actual "refs?arg=$HASH2/test"
'

test_done
