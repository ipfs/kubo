#!/bin/sh
#
# Copyright (c) 2015 Matt Bell
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test HTTP Gateway"

. lib/test-lib.sh

test_init_ipfs
test_config_ipfs_gateway_readonly $ADDR_GWAY
test_launch_ipfs_daemon

port=$PORT_GWAY
apiport=$PORT_API

# TODO check both 5001 and 5002.
# 5001 should have a readable gateway (part of the API)
# 5002 should have a readable gateway (using ipfs config Addresses.Gateway)
# but ideally we should only write the tests once. so maybe we need to
# define a function to test a gateway, and do so for each port.
# for now we check 5001 here as 5002 will be checked in gateway-writable.

# Overwrites: actual.headers, actual.
test_run_curl() {
  rm -f actual.headers actual
  curl -s -D actual.headers -o actual "$@"
}

test_actual_headers_http_status() {
  test_should_contain '^HTTP/1.1 '"$1"'.$' actual.headers
}

test_get_max_age() {
  perl -nwe 's/^Cache-Control:.* max-age=([0-9]+).*\r$/$1/i && print' "$1"
}

test_get_header() {
  perl -nwe 'BEGIN { $header = shift(@ARGV); } s/^\Q$header\E: (.*)\r$/$1/i && print' "$1" "$2"
}

test_item_curl() {
  _item="$1"
  _url="$2"
  _http_status="$3"

  # Just use test_expect_success once #1941 is fixed.
  case "$_url" in
    */ipns/*) TEST_COMMAND=test_expect_success_1941 ;;
    *)        TEST_COMMAND=test_expect_success ;;
  esac

  "$TEST_COMMAND" "$_item: responds with $_http_status" '
    test_run_curl "$_url" &&
    test_actual_headers_http_status "$_http_status"
  '
}

test_item_output() {
  _item="$1"
  _expected_output_file="$2"

  "$TEST_COMMAND" "$_item: output looks good" '
    test_cmp "$_expected_output_file" actual
  '
}

test_item_no_max_age() {
  _item="$1"

  "$TEST_COMMAND" "$_item: has no Cache-Control max-age" '
    test_get_max_age actual.headers >actual.max-age
    test_must_be_empty actual.max-age || test_fsh cat actual.headers
  '
}

test_item_max_age() {
  _item="$1"
  _max_age="$2"

  "$TEST_COMMAND" "$_item: has Cache-Control max-age=$_max_age" '
    test_get_max_age actual.headers >actual.max-age
    printf "%s\n" "$_max_age" >expected.max-age
    test_cmp expected.max-age actual.max-age || test_fsh cat actual.headers
  '
}

test_item_no_etag() {
  _item="$1"

  "$TEST_COMMAND" "$_item: has no ETag" '
    test_get_header ETag actual.headers >actual.etag &&
    test_must_be_empty actual.etag || test_fsh cat actual.headers
  '
}

test_item_etag() {
  _item="$1"
  _etag="$2" # Without the surrounding quote marks

  "$TEST_COMMAND" "$_item: has ETag" '
    test_get_header ETag actual.headers >actual.etag &&
    printf "\"%s\"\n" "$_etag" >expected.etag &&
    test_cmp expected.etag actual.etag || test_fsh cat actual.headers
  '
}

IMMUTABLE_TTL=$((10*365*24*60*60))
UNKNOWN_TTL=60

# DATA1, HASH1: a file
# DATA2, HASH2: a directory
# PEERID: the node ID
test_expect_success "Generating test environment" '
  DATA1=data1 &&
  echo "Hello Worlds!" >"$DATA1" &&
  HASH1=$(ipfs add -q "$DATA1") &&
  DATA2=data2 &&
  mkdir "$DATA2" &&
  echo "12345" >"$DATA2/test" &&
  mkdir "$DATA2/has-index-html" &&
  echo "index" >"$DATA2/has-index-html/index.html" &&
  ipfs add -r -q "$DATA2" >actual &&
  HASH2=$(tail -n 1 actual) &&
  PEERID=$(ipfs config Identity.PeerID) &&
  test_check_peerid "$PEERID"
'

ITEM="GET IPFS path"
test_item_curl    "$ITEM" "http://127.0.0.1:$port/ipfs/$HASH1" "200 OK"
test_item_output  "$ITEM" "$DATA1"
test_item_max_age "$ITEM" "$IMMUTABLE_TTL"
test_item_etag    "$ITEM" "$HASH1"

ITEM="GET IPFS path on API"
test_item_curl       "$ITEM" "http://127.0.0.1:$apiport/ipfs/$HASH1" "403 Forbidden"
test_item_no_max_age "$ITEM"
test_item_no_etag    "$ITEM"

ITEM="GET IPFS directory path"
test_item_curl    "$ITEM" "http://127.0.0.1:$port/ipfs/$HASH2" "200 OK"
test_item_max_age "$ITEM" "$IMMUTABLE_TTL"
test_item_etag    "$ITEM" "$HASH2"
test_expect_success "$ITEM: output looks good" '
  test_should_contain "Index of /ipfs/$HASH2" actual
'

ITEM="GET IPFS directory file"
test_item_curl    "$ITEM" "http://127.0.0.1:$port/ipfs/$HASH2/test" "200 OK"
test_item_output  "$ITEM" "$DATA2/test"
test_item_max_age "$ITEM" "$IMMUTABLE_TTL"
test_item_etag    "$ITEM" "$(ipfs add --only-hash -q "$DATA2/test")"

ITEM="GET IPFS directory path with index.html"
test_item_curl    "$ITEM" "http://127.0.0.1:$port/ipfs/$HASH2/has-index-html" "302 Found"
test_item_max_age "$ITEM" "$IMMUTABLE_TTL"
test_item_etag    "$ITEM" "$(ipfs add --only-hash -r -q "$DATA2/has-index-html" | tail -n 1)"
test_expect_success "$ITEM: redirects to path/" '
  test_get_header Location actual.headers >actual.location &&
  printf "%s\n" "/ipfs/$HASH2/has-index-html/" >expected.location &&
  test_cmp expected.location actual.location || test_fsh cat actual.headers
'

ITEM="GET IPFS directory path/ with index.html"
test_item_curl    "$ITEM" "http://127.0.0.1:$port/ipfs/$HASH2/has-index-html/" "200 OK"
test_item_output  "$ITEM" "$DATA2/has-index-html/index.html"
test_item_max_age "$ITEM" "$IMMUTABLE_TTL"
test_item_etag    "$ITEM" "$(ipfs add --only-hash -r -q "$DATA2/has-index-html" | tail -n 1)"

ITEM="GET IPFS non-existent file"
test_item_curl       "$ITEM" "http://127.0.0.1:$port/ipfs/$HASH2/pleaseDontAddMe" "404 Not Found"
test_item_no_max_age "$ITEM"
test_item_no_etag    "$ITEM"

TTL=10

ITEM="GET IPNS path"
test_expect_success_1941 "$ITEM: IPNS publish with TTL $TTL succeeds" '
  ipfs name publish --ttl="${TTL}s" "$HASH1"
'
test_item_curl    "$ITEM" "http://127.0.0.1:$port/ipns/$PEERID" "200 OK"
test_item_output  "$ITEM" "$DATA1"
test_item_max_age "$ITEM" "$TTL"
test_item_etag    "$ITEM" "$HASH1"

test_timer_start EXPIRY_TIMER "${TTL}s" # The cache entry has expired when this finishes.

ITEM="GET IPNS path again before cache expiry"
# Publish a new version now, the previous version should still be cached.  The
# next item will resolve this version.
test_expect_success_1941 "$ITEM: IPNS publish with default TTL succeeds" '
  ipfs name publish "$HASH2"
'
go-sleep 1s # Ensure the following max-age is strictly less than TTL.
test_item_curl   "$ITEM" "http://127.0.0.1:$port/ipns/$PEERID" "200 OK"
test_item_output "$ITEM" "$DATA1"
test_item_etag   "$ITEM" "$HASH1"
test_expect_success_1941 "$ITEM: has Cache-Control max-age between 0 and $TTL" '
  max_age="$(test_get_max_age actual.headers)" &&
  test "$max_age" -gt 0 &&
  test "$max_age" -lt "$TTL" ||
  test_fsh cat actual.headers
'

# Make sure the expiry timer is still running, otherwise the result might be
# wrong.  If this fails, we will need to increase TTL above to give enough time
# for the tests.
test_expect_success "$ITEM: tests did not take too long" '
  test_timer_is_running "$EXPIRY_TIMER"
'
test_expect_success "$ITEM: previous version is no longer cached" '
  test_timer_wait "$EXPIRY_TIMER"
'

ITEM="GET IPNS path again after cache expiry"
test_item_curl    "$ITEM" "http://127.0.0.1:$port/ipns/$PEERID/test" "200 OK"
test_item_output  "$ITEM" "$DATA2/test"
test_item_max_age "$ITEM" "$UNKNOWN_TTL"
test_item_etag    "$ITEM" "$(ipfs add --only-hash -q "$DATA2/test")"

ITEM="GET invalid IPFS path"
test_item_curl       "$ITEM" "http://127.0.0.1:$port/ipfs/12345" "400 Bad Request"
test_item_no_max_age "$ITEM"
test_item_no_etag    "$ITEM"

ITEM="GET invalid root path"
test_item_curl       "$ITEM" "http://127.0.0.1:$port/12345" "404 Not Found"
test_item_no_max_age "$ITEM"
test_item_no_etag    "$ITEM"

test_expect_success "GET /webui returns code expected" '
  test_curl_resp_http_code "http://127.0.0.1:$apiport/webui" "HTTP/1.1 302 Found" "HTTP/1.1 301 Moved Permanently"
'

test_expect_success "GET /webui/ returns code expected" '
  test_curl_resp_http_code "http://127.0.0.1:$apiport/webui/" "HTTP/1.1 302 Found" "HTTP/1.1 301 Moved Permanently"
'

test_expect_success "GET /logs returns logs" '
	test_expect_code 28 curl http://127.0.0.1:$apiport/logs -m1 > log_out
'

test_expect_success "log output looks good" '
	grep "log API client connected" log_out
'

# test ipfs readonly api

test_curl_gateway_api() {
    curl -sfo actual "http://127.0.0.1:$port/api/v0/$1"
}

test_expect_success "get IPFS directory file through readonly API succeeds" '
  test_curl_gateway_api "cat?arg=$HASH2/test"
'

test_expect_success "get IPFS directory file through readonly API output looks good" '
  test_cmp "$DATA2/test" actual
'

test_expect_success "refs IPFS directory file through readonly API succeeds" '
  test_curl_gateway_api "refs?arg=$HASH2/test"
'

test_expect_success "test gateway api is sanitized" '
for cmd in "add" "block/put" "bootstrap" "config" "dht" "diag" "dns" "get" "id" "mount" "name/publish" "object/put" "object/new" "object/patch" "pin" "ping" "refs/local" "repo" "resolve" "stats" "swarm" "tour" "file" "update" "version" "bitswap"; do
    test_curl_resp_http_code "http://127.0.0.1:$port/api/v0/$cmd" "HTTP/1.1 404 Not Found"
  done
'

test_kill_ipfs_daemon

test_done
