#!/usr/bin/env bash

test_description="Test HTTP Gateway IPNS Record (application/vnd.ipfs.ipns-record) Support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

# Import test case
# See the static fixtures in ./t0124-gateway-ipns-record/
IPNS_KEY=k51qzi5uqu5dh71qgwangrt6r0nd4094i88nsady6qgd1dhjcyfsaqmpp143ab
FILE_CID=bafkreidfdrlkeq4m4xnxuyx6iae76fdm4wgl5d4xzsb77ixhyqwumhz244 # A file containing Hello IPFS
test_expect_success "Add the test directory & IPNS records" '
  ipfs dag import ../t0124-gateway-ipns-record/fixtures.car &&
  ipfs routing put /ipns/${IPNS_KEY} ../t0124-gateway-ipns-record/${IPNS_KEY}.ipns-record
'

test_expect_success "Create and Publish IPNS Key" '
  curl "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_KEY" > curl_output_filename &&
  test_should_contain "Hello IPFS" curl_output_filename
'

test_expect_success "GET KEY with format=ipns-record and validate key" '
  curl "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_KEY?format=ipns-record" > curl_output_filename &&
  ipfs name inspect --verify $IPNS_KEY < curl_output_filename > verify_output &&
  test_should_contain "$FILE_CID" verify_output
'

test_expect_success "GET KEY with 'Accept: application/vnd.ipfs.ipns-record' and validate key" '
  curl -H "Accept: application/vnd.ipfs.ipns-record" "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_KEY" > curl_output_filename &&
  ipfs name inspect --verify $IPNS_KEY < curl_output_filename > verify_output &&
  test_should_contain "$FILE_CID" verify_output
'

test_expect_success "GET KEY with format=ipns-record has expected HTTP headers" '
  curl -sD - "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_KEY?format=ipns-record" > curl_output_filename 2>&1 &&
  test_should_contain "Content-Disposition: attachment;" curl_output_filename &&
  test_should_contain "Content-Type: application/vnd.ipfs.ipns-record" curl_output_filename &&
  test_should_contain "Cache-Control: public, max-age=3155760000" curl_output_filename
'

test_expect_success "GET KEY with 'Accept: application/vnd.ipfs.ipns-record' has expected HTTP headers" '
  curl -H "Accept: application/vnd.ipfs.ipns-record" -sD - "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_KEY" > curl_output_filename 2>&1 &&
  test_should_contain "Content-Disposition: attachment;" curl_output_filename &&
  test_should_contain "Content-Type: application/vnd.ipfs.ipns-record" curl_output_filename &&
  test_should_contain "Cache-Control: public, max-age=3155760000" curl_output_filename
'

test_expect_success "GET KEY with expliciy ?filename= succeeds with modified Content-Disposition header" '
  curl -sD - "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_KEY?format=ipns-record&filename=testтест.ipns-record" > curl_output_filename 2>&1 &&
  grep -F "Content-Disposition: attachment; filename=\"test____.ipns-record\"; filename*=UTF-8'\'\''test%D1%82%D0%B5%D1%81%D1%82.ipns-record" curl_output_filename &&
  test_should_contain "Content-Type: application/vnd.ipfs.ipns-record" curl_output_filename
'

test_kill_ipfs_daemon

test_done
