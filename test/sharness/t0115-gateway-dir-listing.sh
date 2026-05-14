#!/usr/bin/env bash
#
# Copyright (c) Protocol Labs

test_description="Test directory listing (dir-index-html) on the HTTP gateway"


. lib/test-lib.sh

## ============================================================================
## Start IPFS Node and prepare test CIDs
## ============================================================================

test_expect_success "ipfs init" '
  export IPFS_PATH="$(pwd)/.ipfs" &&
  ipfs init --profile=test > /dev/null
'

test_launch_ipfs_daemon_without_network

# Import test case
# See the static fixtures in ./t0115-gateway-dir-listing/
test_expect_success "Add the test directory" '
  ipfs dag import --pin-roots ../t0115-gateway-dir-listing/fixtures.car
'
DIR_CID=bafybeig6ka5mlwkl4subqhaiatalkcleo4jgnr3hqwvpmsqfca27cijp3i # ./rootDir/
FILE_CID=bafkreialihlqnf5uwo4byh4n3cmwlntwqzxxs2fg5vanqdi3d7tb2l5xkm # ./rootDir/ą/ę/file-źł.txt
FILE_SIZE=34

## ============================================================================
## Test dir listing on path gateway (eg. 127.0.0.1:8080/ipfs/)
## ============================================================================

test_expect_success "path gw: backlink on root CID should be hidden" '
  curl -sD - http://127.0.0.1:$GWAY_PORT/ipfs/${DIR_CID}/ > list_response &&
  test_should_contain "Index of" list_response &&
  test_should_not_contain "<a href=\"/ipfs/$DIR_CID/\">..</a>" list_response
'

test_expect_success "path gw: redirect dir listing to URL with trailing slash" '
  curl -sD - http://127.0.0.1:$GWAY_PORT/ipfs/${DIR_CID}/ą/ę > list_response &&
  test_should_contain "HTTP/1.1 301 Moved Permanently" list_response &&
  test_should_contain "Location: /ipfs/${DIR_CID}/%C4%85/%C4%99/" list_response
'

test_expect_success "path gw: Etag should be present" '
  curl -sD - http://127.0.0.1:$GWAY_PORT/ipfs/${DIR_CID}/ą/ę/ > list_response &&
  test_should_contain "Index of" list_response &&
  test_should_contain "Etag: \"DirIndex-" list_response
'

test_expect_success "path gw: breadcrumbs should point at /ipfs namespace mounted at Origin root" '
  test_should_contain "/ipfs/<a href=\"/ipfs/$DIR_CID\">$DIR_CID</a>/<a href=\"/ipfs/$DIR_CID/%C4%85\">ą</a>/<a href=\"/ipfs/$DIR_CID/%C4%85/%C4%99\">ę</a>" list_response
'

test_expect_success "path gw: backlink on subdirectory should point at parent directory" '
  test_should_contain "<a href=\"/ipfs/$DIR_CID/%C4%85/%C4%99/..\">..</a>" list_response
'

test_expect_success "path gw: name column should be a link to its content path" '
  test_should_contain "<a href=\"/ipfs/$DIR_CID/%C4%85/%C4%99/file-%C5%BA%C5%82.txt\">file-źł.txt</a>" list_response
'

test_expect_success "path gw: hash column should be a CID link with filename param" '
  test_should_contain "<a class=\"ipfs-hash\" translate=\"no\" href=\"/ipfs/$FILE_CID?filename=file-%25C5%25BA%25C5%2582.txt\">" list_response
'

## ============================================================================
## Test dir listing on subdomain gateway (eg. <cid>.ipfs.localhost:8080)
## ============================================================================

DIR_HOSTNAME="${DIR_CID}.ipfs.localhost"
# note: we skip DNS lookup by running curl with --resolve $DIR_HOSTNAME:127.0.0.1

test_expect_success "subdomain gw: backlink on root CID should be hidden" '
  curl -sD - --resolve $DIR_HOSTNAME:$GWAY_PORT:127.0.0.1 http://$DIR_HOSTNAME:$GWAY_PORT/ > list_response &&
  test_should_contain "Index of" list_response &&
  test_should_not_contain "<a href=\"/\">..</a>" list_response
'

test_expect_success "subdomain gw: redirect dir listing to URL with trailing slash" '
  curl -sD - --resolve $DIR_HOSTNAME:$GWAY_PORT:127.0.0.1 http://$DIR_HOSTNAME:$GWAY_PORT/ą/ę > list_response &&
  test_should_contain "HTTP/1.1 301 Moved Permanently" list_response &&
  test_should_contain "Location: /%C4%85/%C4%99/" list_response
'

test_expect_success "subdomain gw: Etag should be present" '
  curl -sD - --resolve $DIR_HOSTNAME:$GWAY_PORT:127.0.0.1 http://$DIR_HOSTNAME:$GWAY_PORT/ą/ę/ > list_response &&
  test_should_contain "Index of" list_response &&
  test_should_contain "Etag: \"DirIndex-" list_response
'

test_expect_success "subdomain gw: backlink on subdirectory should point at parent directory" '
  test_should_contain "<a href=\"/%C4%85/%C4%99/..\">..</a>" list_response
'

test_expect_success "subdomain gw: breadcrumbs should leverage path-based router mounted on the parent domain" '
  test_should_contain "/ipfs/<a href=\"//localhost:$GWAY_PORT/ipfs/$DIR_CID\">$DIR_CID</a>/<a href=\"//localhost:$GWAY_PORT/ipfs/$DIR_CID/%C4%85\">ą</a>/<a href=\"//localhost:$GWAY_PORT/ipfs/$DIR_CID/%C4%85/%C4%99\">ę</a>" list_response
'

test_expect_success "subdomain gw: name column should be a link to content root mounted at subdomain origin" '
  test_should_contain "<a href=\"/%C4%85/%C4%99/file-%C5%BA%C5%82.txt\">file-źł.txt</a>" list_response
'

test_expect_success "subdomain gw: hash column should be a CID link to path router with filename param" '
  test_should_contain "<a class=\"ipfs-hash\" translate=\"no\" href=\"//localhost:$GWAY_PORT/ipfs/$FILE_CID?filename=file-%25C5%25BA%25C5%2582.txt\">" list_response
'

## ============================================================================
## Test dir listing on DNSLink gateway (eg. example.com)
## ============================================================================

# DNSLink test requires a daemon in online mode with precached /ipns/ mapping
test_kill_ipfs_daemon
DNSLINK_HOSTNAME="website.example.com"
export IPFS_NS_MAP="$DNSLINK_HOSTNAME:/ipfs/$DIR_CID"
test_launch_ipfs_daemon

# Note that:
# - this type of gateway is also tested in gateway_test.go#TestIPNSHostnameBacklinks
#   (go tests and sharness tests should be kept in sync)
# - we skip DNS lookup by running curl with --resolve $DNSLINK_HOSTNAME:127.0.0.1

test_expect_success "dnslink gw: backlink on root CID should be hidden" '
  curl -v -sD - --resolve $DNSLINK_HOSTNAME:$GWAY_PORT:127.0.0.1 http://$DNSLINK_HOSTNAME:$GWAY_PORT/ > list_response &&
  test_should_contain "Index of" list_response &&
  test_should_not_contain "<a href=\"/\">..</a>" list_response
'

test_expect_success "dnslink gw: redirect dir listing to URL with trailing slash" '
  curl -sD - --resolve $DNSLINK_HOSTNAME:$GWAY_PORT:127.0.0.1 http://$DNSLINK_HOSTNAME:$GWAY_PORT/ą/ę > list_response &&
  test_should_contain "HTTP/1.1 301 Moved Permanently" list_response &&
  test_should_contain "Location: /%C4%85/%C4%99/" list_response
'

test_expect_success "dnslink gw: Etag should be present" '
  curl -sD - --resolve $DNSLINK_HOSTNAME:$GWAY_PORT:127.0.0.1 http://$DNSLINK_HOSTNAME:$GWAY_PORT/ą/ę/ > list_response &&
  test_should_contain "Index of" list_response &&
  test_should_contain "Etag: \"DirIndex-" list_response
'

test_expect_success "dnslink gw: backlink on subdirectory should point at parent directory" '
  test_should_contain "<a href=\"/%C4%85/%C4%99/..\">..</a>" list_response
'

test_expect_success "dnslink gw: breadcrumbs should point at content root mounted at dnslink origin" '
  test_should_contain "/ipns/<a href=\"//$DNSLINK_HOSTNAME:$GWAY_PORT/\">website.example.com</a>/<a href=\"//$DNSLINK_HOSTNAME:$GWAY_PORT/%C4%85\">ą</a>/<a href=\"//$DNSLINK_HOSTNAME:$GWAY_PORT/%C4%85/%C4%99\">ę</a>" list_response
'

test_expect_success "dnslink gw: name column should be a link to content root mounted at dnslink origin" '
  test_should_contain "<a href=\"/%C4%85/%C4%99/file-%C5%BA%C5%82.txt\">file-źł.txt</a>" list_response
'

# DNSLink websites don't have public gateway mounted by default
# See: https://github.com/ipfs/dir-index-html/issues/42
test_expect_success "dnslink gw: hash column should be a CID link to cid.ipfs.tech" '
  test_should_contain "<a class=\"ipfs-hash\" translate=\"no\" href=\"https://cid.ipfs.tech/#$FILE_CID\" target=\"_blank\" rel=\"noreferrer noopener\">" list_response
'

## ============================================================================
## End of tests, cleanup
## ============================================================================

test_kill_ipfs_daemon
test_expect_success "clean up ipfs dir" '
  rm -rf "$IPFS_PATH"
'
test_done
