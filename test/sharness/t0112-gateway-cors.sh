#!/usr/bin/env bash

test_description="Test CORS behavior on Gateway port"

. lib/test-lib.sh

test_init_ipfs

# Default config
test_expect_success "Default Gateway.HTTPHeaders config match expected values" '
cat <<EOF > expected
{
  "Access-Control-Allow-Headers": [
    "X-Requested-With",
    "Range",
    "User-Agent"
  ],
  "Access-Control-Allow-Methods": [
    "GET"
  ],
  "Access-Control-Allow-Origin": [
    "*"
  ]
}
EOF
    ipfs config --json Gateway.HTTPHeaders > actual &&
    test_cmp expected actual
'

test_launch_ipfs_daemon

thash='bafkqabtimvwgy3yk' # hello

# Gateway

# HTTP GET Request
test_expect_success "GET to Gateway succeeds" '
  curl -svX GET -H "Origin: https://example.com" "http://127.0.0.1:$GWAY_PORT/ipfs/$thash" >/dev/null 2>curl_output &&
  cat curl_output
'

# GET Response from Gateway should contain CORS headers
test_expect_success "GET response for Gateway resource looks good" '
  test_should_contain "< Access-Control-Allow-Origin: \*" curl_output &&
  test_should_contain "< Access-Control-Allow-Methods: GET" curl_output &&
  test_should_contain "< Access-Control-Allow-Headers: Range" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: Content-Range" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: Content-Length" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: X-Ipfs-Path" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: X-Ipfs-Roots" curl_output
'

# HTTP OPTIONS Request
test_expect_success "OPTIONS to Gateway succeeds" '
  curl -svX OPTIONS -H "Origin: https://example.com" "http://127.0.0.1:$GWAY_PORT/ipfs/$thash" 2>curl_output &&
  cat curl_output
'

# OPTION Response from Gateway should contain CORS headers
test_expect_success "OPTIONS response for Gateway resource looks good" '
  test_should_contain "< Access-Control-Allow-Origin: \*" curl_output &&
  test_should_contain "< Access-Control-Allow-Methods: GET" curl_output &&
  test_should_contain "< Access-Control-Allow-Headers: Range" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: Content-Range" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: Content-Length" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: X-Ipfs-Path" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: X-Ipfs-Roots" curl_output
'

test_kill_ipfs_daemon

# Test CORS safelisting of custom headers
test_expect_success "Can configure gateway headers" '
  ipfs config --json Gateway.HTTPHeaders.Access-Control-Allow-Headers "[\"X-Custom1\"]" &&
  ipfs config --json Gateway.HTTPHeaders.Access-Control-Expose-Headers "[\"X-Custom2\"]" &&
  ipfs config --json Gateway.HTTPHeaders.Access-Control-Allow-Origin "[\"localhost\"]"
'

test_launch_ipfs_daemon

test_expect_success "OPTIONS to Gateway without custom headers succeeds" '
  curl -svX OPTIONS -H "Origin: https://example.com" "http://127.0.0.1:$GWAY_PORT/ipfs/$thash" 2>curl_output &&
  cat curl_output
'
# Range and Content-Range are safelisted by default, and keeping them makes better devexp
# because it does not cause regressions in range requests made by JS
test_expect_success "Access-Control-Allow-Headers extends the implicit list" '
  test_should_contain "< Access-Control-Allow-Headers: Range" curl_output &&
  test_should_contain "< Access-Control-Allow-Headers: X-Custom1" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: Content-Range" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: Content-Length" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: X-Ipfs-Path" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: X-Ipfs-Roots" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: X-Custom2" curl_output
'

test_expect_success "OPTIONS to Gateway with a custom header succeeds" '
  curl -svX OPTIONS -H "Origin: https://example.com" -H "Access-Control-Request-Headers: X-Unexpected-Custom" "http://127.0.0.1:$GWAY_PORT/ipfs/$thash" 2>curl_output &&
  cat curl_output
'
test_expect_success "Access-Control-Allow-Headers extends the implicit list" '
  test_should_not_contain "< Access-Control-Allow-Headers: X-Unexpected-Custom" curl_output &&
  test_should_contain "< Access-Control-Allow-Headers: Range" curl_output &&
  test_should_contain "< Access-Control-Allow-Headers: X-Custom1" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: Content-Range" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: X-Custom2" curl_output
'

# Origin is sensitive security perimeter, and we assume override should remove
# any implicit records
test_expect_success "Access-Control-Allow-Origin replaces the implicit list" '
  test_should_contain "< Access-Control-Allow-Origin: localhost" curl_output
'

# Read-Only /api/v0 RPC API (legacy subset, exposed on the Gateway Port)
# TODO: we want to remove it, but for now this guards the legacy behavior to not go any further

# also check this, as due to legacy reasons Kubo exposes small subset of /api/v0 on GW port
test_expect_success "Assert the default API.HTTPHeaders config is empty" '
    echo "{}" > expected &&
    ipfs config --json API.HTTPHeaders > actual &&
    test_cmp expected actual
'

# HTTP GET Request
test_expect_success "Default CORS GET to {gw}/api/v0" '
  curl -svX GET -H "Origin: https://example.com" "http://127.0.0.1:$GWAY_PORT/api/v0/cat?arg=$thash" >/dev/null 2>curl_output
'
test_expect_success "Default CORS GET response from {gw}/api/v0 is 403 Forbidden and has no CORS headers" '
  test_should_contain "HTTP/1.1 403 Forbidden" curl_output &&
  test_should_not_contain "< Access-Control-" curl_output
'

# HTTP OPTIONS Request
test_expect_success "Default OPTIONS to {gw}/api/v0" '
  curl -svX OPTIONS -H "Origin: https://example.com" "http://127.0.0.1:$GWAY_PORT/api/v0/cat?arg=$thash" 2>curl_output
'
# OPTIONS Response from the API should NOT contain CORS headers
test_expect_success "OPTIONS response from {gw}/api/v0 has no CORS header" '
  test_should_not_contain "< Access-Control-" curl_output
'

test_kill_ipfs_daemon

# TODO: /api/v0 with CORS headers set in API.HTTPHeaders  does not really work,
# as not all headers are correctly set. Below is only a basic regression test that documents
# current state. Fixing CORS on /api/v0 (RPC and Gateway port) is tracked in https://github.com/ipfs/kubo/issues/7667

test_expect_success "Manually set API.HTTPHeaders config to be as relaxed as Gateway.HTTPHeaders" "
  ipfs config --json API.HTTPHeaders.Access-Control-Allow-Origin '[\"https://example.com\"]'
"
# TODO: ipfs config --json API.HTTPHeaders.Access-Control-Allow-Methods '[\"GET\",\"POST\"]' &&
# TODO: ipfs config --json API.HTTPHeaders.Access-Control-Allow-Headers '[\"X-Requested-With\", \"Range\", \"User-Agent\"]'

test_launch_ipfs_daemon

# HTTP GET Request
test_expect_success "Manually relaxed CORS GET to {gw}/api/v0" '
  curl -svX GET -H "Origin: https://example.com" "http://127.0.0.1:$GWAY_PORT/api/v0/cat?arg=$thash" >/dev/null 2>curl_output
'
test_expect_success "Manually relaxed CORS GET response from {gw}/api/v0 is the same as Gateway" '
  test_should_contain "HTTP/1.1 200 OK" curl_output &&
  test_should_contain "< Access-Control-Allow-Origin: https://example.com" curl_output
'
# TODO: test_should_contain "< Access-Control-Allow-Methods: GET" curl_output

# HTTP OPTIONS Request
test_expect_success "Manually relaxed OPTIONS to {gw}/api/v0" '
  curl -svX OPTIONS -H "Origin: https://example.com" "http://127.0.0.1:$GWAY_PORT/api/v0/cat?arg=$thash" 2>curl_output
'
# OPTIONS Response from the API should NOT contain CORS headers
test_expect_success "Manually relaxed OPTIONS response from {gw}/api/v0 is the same as Gateway" '
  test_should_contain "< Access-Control-Allow-Origin: https://example.com" curl_output
'
# TODO: test_should_contain "< Access-Control-Allow-Methods: GET" curl_output

test_kill_ipfs_daemon

test_done
