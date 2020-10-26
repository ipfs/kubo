#!/usr/bin/env bash
#
# Copyright (c) 2020 Protocol Labs
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test API browser security"

. lib/test-lib.sh

test_init_ipfs

PEERID=$(ipfs config Identity.PeerID)

test_launch_ipfs_daemon

test_expect_success "browser is unable to access API without Origin" '
  curl -sD - -X POST -A "Mozilla" "http://127.0.0.1:$API_PORT/api/v0/id" >curl_output &&
  grep "HTTP/1.1 403 Forbidden" curl_output
'

test_expect_success "browser is unable to access API with invalid Origin" '
  curl -sD - -X POST -A "Mozilla" -H "Origin: https://invalid.example.com" "http://127.0.0.1:$API_PORT/api/v0/id" >curl_output &&
  grep "HTTP/1.1 403 Forbidden" curl_output
'

test_expect_success "browser is able to access API if Origin is the API port on localhost (ipv4)" '
  curl -sD - -X POST -A "Mozilla" -H "Origin: http://127.0.0.1:$API_PORT" "http://127.0.0.1:$API_PORT/api/v0/id" >curl_output &&
  grep "HTTP/1.1 200 OK" curl_output && grep "$PEERID" curl_output
'

test_expect_success "browser is able to access API if Origin is the API port on localhost (ipv6)" '
  curl -sD - -X POST -A "Mozilla" -H "Origin: http://[::1]:$API_PORT" "http://127.0.0.1:$API_PORT/api/v0/id" >curl_output &&
  grep "HTTP/1.1 200 OK" curl_output && grep "$PEERID" curl_output
'

test_expect_success "browser is able to access API if Origin is the API port on localhost (localhost name)" '
  curl -sD - -X POST -A "Mozilla" -H "Origin: http://localhost:$API_PORT" "http://127.0.0.1:$API_PORT/api/v0/id" >curl_output &&
  grep "HTTP/1.1 200 OK" curl_output && grep "$PEERID" curl_output
'

test_kill_ipfs_daemon

test_expect_success "setting CORS in API.HTTPHeaders works via CLI" "
  ipfs config --json API.HTTPHeaders.Access-Control-Allow-Origin '[\"https://valid.example.com\"]' &&
  ipfs config --json API.HTTPHeaders.Access-Control-Allow-Methods '[\"POST\"]' &&
  ipfs config --json API.HTTPHeaders.Access-Control-Allow-Headers '[\"X-Requested-With\"]'
"

test_launch_ipfs_daemon

# https://developer.mozilla.org/en-US/docs/Glossary/Preflight_request
test_expect_success "OPTIONS with preflight request to API with CORS allowlist succeeds" '
  curl -svX OPTIONS -A "Mozilla" -H "Origin: https://valid.example.com" -H "Access-Control-Request-Method: POST" -H "Access-Control-Request-Headers: origin, x-requested-with" "http://127.0.0.1:$API_PORT/api/v0/id" 2>curl_output &&
  cat curl_output
'

# OPTION Response from Gateway should contain CORS headers, otherwise JS won't work
test_expect_success "OPTIONS response for API with CORS allowslist looks good" '
  grep "< Access-Control-Allow-Origin: https://valid.example.com" curl_output
'

test_expect_success "browser is able to access API with valid Origin matching CORS allowlist" '
  curl -sD - -X POST -A "Mozilla" -H "Origin: https://valid.example.com" "http://127.0.0.1:$API_PORT/api/v0/id" >curl_output &&
  grep "HTTP/1.1 200 OK" curl_output && grep "$PEERID" curl_output
'

test_kill_ipfs_daemon
test_done
