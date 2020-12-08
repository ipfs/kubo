#!/usr/bin/env bash
#
# Copyright (c) 2016 Marcin Rataj
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test HTTP Gateway CORS Support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

thash='QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn'

# Gateway

# HTTP GET Request
test_expect_success "GET to Gateway succeeds" '
  curl -svX GET "http://127.0.0.1:$GWAY_PORT/ipfs/$thash" >/dev/null 2>curl_output &&
  cat curl_output
'

# GET Response from Gateway should contain CORS headers
test_expect_success "GET response for Gateway resource looks good" '
  grep "< Access-Control-Allow-Origin: \*" curl_output &&
  grep "< Access-Control-Allow-Methods: GET" curl_output &&
  grep "< Access-Control-Allow-Headers: Range" curl_output &&
  grep "< Access-Control-Expose-Headers: Content-Range" curl_output
'

# HTTP OPTIONS Request
test_expect_success "OPTIONS to Gateway succeeds" '
  curl -svX OPTIONS "http://127.0.0.1:$GWAY_PORT/ipfs/$thash" 2>curl_output &&
  cat curl_output
'

# OPTION Response from Gateway should contain CORS headers
test_expect_success "OPTIONS response for Gateway resource looks good" '
  grep "< Access-Control-Allow-Origin: \*" curl_output &&
  grep "< Access-Control-Allow-Methods: GET" curl_output &&
  grep "< Access-Control-Allow-Headers: Range" curl_output &&
  grep "< Access-Control-Expose-Headers: Content-Range" curl_output
'

test_kill_ipfs_daemon

# Change headers
test_expect_success "Can configure gateway headers" '
  ipfs config --json Gateway.HTTPHeaders.Access-Control-Allow-Headers "[\"X-Custom1\"]" &&
  ipfs config --json Gateway.HTTPHeaders.Access-Control-Expose-Headers "[\"X-Custom2\"]" &&
  ipfs config --json Gateway.HTTPHeaders.Access-Control-Allow-Origin "[\"localhost\"]"
'

test_launch_ipfs_daemon

test_expect_success "OPTIONS to Gateway succeeds" '
  curl -svX OPTIONS "http://127.0.0.1:$GWAY_PORT/ipfs/$thash" 2>curl_output &&
  cat curl_output
'

test_expect_success "Access-Control-Allow-Headers extends" '
  grep "< Access-Control-Allow-Headers: Range" curl_output &&
  grep "< Access-Control-Allow-Headers: X-Custom1" curl_output &&
  grep "< Access-Control-Expose-Headers: Content-Range" curl_output &&
  grep "< Access-Control-Expose-Headers: X-Custom2" curl_output
'

test_expect_success "Access-Control-Allow-Origin replaces" '
  grep "< Access-Control-Allow-Origin: localhost" curl_output
'

# Read-Only API (at the Gateway Port)

# HTTP GET Request
test_expect_success "GET to API succeeds" '
  curl -svX GET "http://127.0.0.1:$GWAY_PORT/api/v0/cat?arg=$thash" >/dev/null 2>curl_output
'
# GET Response from the API should NOT contain CORS headers
# Blacklisting: https://git.io/vzaj2
# Rationale: https://git.io/vzajX
test_expect_success "OPTIONS response for API looks good" '
  grep -q "Access-Control-Allow-" curl_output && false || true
'

# HTTP OPTIONS Request
test_expect_success "OPTIONS to API succeeds" '
  curl -svX OPTIONS "http://127.0.0.1:$GWAY_PORT/api/v0/cat?arg=$thash" 2>curl_output
'
# OPTIONS Response from the API should NOT contain CORS headers
test_expect_success "OPTIONS response for API looks good" '
  grep -q "Access-Control-Allow-" curl_output && false || true
'

test_kill_ipfs_daemon

test_done
