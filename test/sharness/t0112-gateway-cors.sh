#!/usr/bin/env bash

test_description="Test CORS behavior on Gateway port"

. lib/test-lib.sh

test_init_ipfs

# Default config
test_expect_success "Default Gateway.HTTPHeaders is empty (implicit CORS values from boxo/gateway)" '
cat <<EOF > expected
{}
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
  test_should_contain "< Access-Control-Allow-Methods: HEAD" curl_output &&
  test_should_contain "< Access-Control-Allow-Methods: OPTIONS" curl_output &&
  test_should_contain "< Access-Control-Allow-Headers: Content-Type" curl_output &&
  test_should_contain "< Access-Control-Allow-Headers: Range" curl_output &&
  test_should_contain "< Access-Control-Allow-Headers: User-Agent" curl_output &&
  test_should_contain "< Access-Control-Allow-Headers: X-Requested-With" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: Content-Range" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: Content-Length" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: X-Chunked-Output" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: X-Stream-Output" curl_output &&
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
  test_should_contain "< Access-Control-Allow-Methods: HEAD" curl_output &&
  test_should_contain "< Access-Control-Allow-Methods: OPTIONS" curl_output &&
  test_should_contain "< Access-Control-Allow-Headers: Content-Type" curl_output &&
  test_should_contain "< Access-Control-Allow-Headers: Range" curl_output &&
  test_should_contain "< Access-Control-Allow-Headers: User-Agent" curl_output &&
  test_should_contain "< Access-Control-Allow-Headers: X-Requested-With" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: Content-Range" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: Content-Length" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: X-Chunked-Output" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: X-Stream-Output" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: X-Ipfs-Path" curl_output &&
  test_should_contain "< Access-Control-Expose-Headers: X-Ipfs-Roots" curl_output
'

# HTTP OPTIONS Request on path → subdomain HTTP 301 redirect
# (regression test for https://github.com/ipfs/kubo/issues/9983#issuecomment-1599673976)
test_expect_success "OPTIONS to Gateway succeeds" '
  curl -svX OPTIONS -H "Origin: https://example.com" "http://localhost:$GWAY_PORT/ipfs/$thash" 2>curl_output &&
  cat curl_output
'
# OPTION Response from Gateway should contain CORS headers
test_expect_success "OPTIONS response for subdomain redirect looks good" '
  test_should_contain "HTTP/1.1 301 Moved Permanently" curl_output &&
  test_should_contain "Location" curl_output &&
  test_should_contain "< Access-Control-Allow-Origin: \*" curl_output &&
  test_should_contain "< Access-Control-Allow-Methods: GET" curl_output
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

test_kill_ipfs_daemon

test_done
