#!/usr/bin/env bash
#
# Copyright (c) 2016 Lars Gierth
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test API security"

. lib/test-lib.sh

test_init_ipfs

# Import test case
# See the static fixtures in ./t0400-api-no-gateway/
test_expect_success "Add the test directory" '
  ipfs dag import ../t0400-api-no-gateway/fixtures.car
'
HASH=QmNYERzV2LfD2kkfahtfv44ocHzEFK1sLBaE7zdcYT2GAZ # a file containing the string "testing"

# by default, we don't let you load arbitrary ipfs objects through the api,
# because this would open up the api to scripting vulnerabilities.
# only the webui objects are allowed.
# if you know what you're doing, go ahead and pass --unrestricted-api.

test_launch_ipfs_daemon
test_expect_success "Gateway on API unavailable" '
  test_curl_resp_http_code "http://127.0.0.1:$API_PORT/ipfs/$HASH" "HTTP/1.1 404 Not Found"
'
test_kill_ipfs_daemon

test_launch_ipfs_daemon --unrestricted-api
test_expect_success "Gateway on --unrestricted-api API available" '
  test_curl_resp_http_code "http://127.0.0.1:$API_PORT/ipfs/$HASH" "HTTP/1.1 200 OK"
'
test_kill_ipfs_daemon

test_done
