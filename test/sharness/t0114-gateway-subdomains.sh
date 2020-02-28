#!/usr/bin/env bash
#
# Copyright (c) Protocol Labs

test_description="Test subdomain support on the HTTP gateway"

. lib/test-lib.sh

# Helper that tests gateway response over direct HTTP
# and in all supported HTTP proxy modes
test_localhost_gateway_response_should_contain() {
  # regular HTTP request
  test_expect_success "$1 (direct HTTP)" "
    curl -sD - \"$2\" > response &&
    test_should_contain \"$3\" response
  "
  # HTTP proxy
  test_expect_success "$1 (HTTP proxy)" "
    curl -x http://127.0.0.1:$GWAY_PORT -sD - \"$2\" > response &&
    test_should_contain \"$3\" response
  "
  # HTTP proxy 1.0
  test_expect_success "$1 (HTTP proxy 1.0)" "
    curl --proxy1.0 http://127.0.0.1:$GWAY_PORT -sD - \"$2\" > response &&
    test_should_contain \"$3\" response
  "
  # TODO: HTTP proxy tunneling (CONNECT)
  # https://tools.ietf.org/html/rfc7231#section-4.3.6
  # In HTTP/1.x, the pseudo-method CONNECT
  # can be used to convert an HTTP connection into a tunnel to a remote host
  test_expect_success "$1 (HTTP proxy tunneling)" "
    curl --proxytunnel -x http://127.0.0.1:$GWAY_PORT -sD - \"$2\" > response &&
    test_should_contain \"$3\" response
  "
}

# Helper that checks gateway resonse for specific hostname in Host header
test_hostname_gateway_response_should_contain() {
  test_expect_success "$1" "
    curl -H \"Host: $2\" -sD - \"$3\" > response &&
    test_should_contain \"$4\" response
  "
}

test_init_ipfs
test_launch_ipfs_daemon --offline

# CIDv0to1 is necessary because raw-leaves are enabled by default during
# "ipfs add" with CIDv1 and disabled with CIDv0
test_expect_success "Add test text file" '
  CIDv1=$(echo "hello" | ipfs add --cid-version 1 -Q)
  CIDv0=$(echo "hello" | ipfs add --cid-version 0 -Q)
  CIDv0to1=$(echo "$CIDv0" | ipfs cid base32)
'

test_expect_success "Publish test text file to IPNS" '
  PEERID=$(ipfs id --format="<id>")
  IPNS_IDv0=$(echo "$PEERID" | ipfs cid format -v 0)
  IPNS_IDv1=$(echo "$PEERID" | ipfs cid format -v 1 --codec libp2p-key -b base32)
  IPNS_IDv1_DAGPB=$(echo "$IPNS_IDv0" | ipfs cid format -v 1 -b base32)
  test_check_peerid "${PEERID}" &&
  ipfs name publish --allow-offline -Q "/ipfs/$CIDv1" > name_publish_out &&
  ipfs name resolve "$PEERID"  > output &&
  printf "/ipfs/%s\n" "$CIDv1" > expected2 &&
  test_cmp expected2 output
'
#test_kill_ipfs_daemon
#test_launch_ipfs_daemon

## ============================================================================
## Test path-based requests to a local gateway with default config
## (forced redirects to http://*.localhost)
## ============================================================================

# /ipfs/<cid>

# IP remains old school path-based gateway

test_localhost_gateway_response_should_contain \
  "Request for 127.0.0.1/ipfs/{CID} stays on path" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "hello"

# 'localhost' hostname is used for subdomains, and should not return
#  payload directly, but redirect to URL with proper origin isolation

test_localhost_gateway_response_should_contain \
  "Request for localhost/ipfs/{CIDv1} redirects to subdomain" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv1" \
  "Location: http://$CIDv1.ipfs.localhost:$GWAY_PORT/"

test_localhost_gateway_response_should_contain \
  "Request for localhost/ipfs/{CIDv0} redirects to CIDv1 representation in subdomain" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv0" \
  "Location: http://${CIDv0to1}.ipfs.localhost:$GWAY_PORT/"

# /ipns/<libp2p-key>

test_localhost_gateway_response_should_contain \
  "Request for localhost/ipns/{CIDv0} redirects to CIDv1 with libp2p-key multicodec in subdomain" \
  "http://localhost:$GWAY_PORT/ipns/$IPNS_IDv0"
  "Location: http://${IPNS_IDv1}.ipns.localhost:$GWAY_PORT/"

# /ipns/<dnslink-fqdn>

test_localhost_gateway_response_should_contain \
  "Request for localhost/ipns/{fqdn} redirects to DNSLink in subdomain" \
  "http://localhost:$GWAY_PORT/ipns/en.wikipedia-on-ipfs.org/wiki" \
  "Location: http://en.wikipedia-on-ipfs.org.ipns.localhost:$GWAY_PORT/wiki"

## ============================================================================
## Test subdomain-based requests to a local gateway with default config
## (origin per content root at http://*.localhost)
## ============================================================================

# {CID}.ipfs.localhost

test_localhost_gateway_response_should_contain \
  "Request for {CID}.ipfs.localhost should return expected payload" \
  "http://${CIDv1}.ipfs.localhost:$GWAY_PORT" \
  "hello"

test_localhost_gateway_response_should_contain \
  "Request for {CID}.ipfs.localhost/ipfs/{CID} should return HTTP 404" \
  "http://${CIDv1}.ipfs.localhost:$GWAY_PORT/ipfs/$CIDv1" \
  "404 Not Found"

# {CID}.ipfs.localhost/sub/dir (Directory Listing)

test_expect_success "Add the test directory" '
  mkdir -p testdirlisting/subdir1/subdir2 &&
  echo "hello" > testdirlisting/hello &&
  echo "subdir2-bar" > testdirlisting/subdir1/subdir2/bar &&
  DIR_CID=$(ipfs add -Qr --cid-version 1 testdirlisting)
'

test_expect_success "Valid file and subdirectory paths in directory listing at {cid}.ipfs.localhost" '
  curl -s "http://${DIR_CID}.ipfs.localhost:$GWAY_PORT" > list_response &&
  test_should_contain "<a href=\"/hello\">hello</a>" list_response &&
  test_should_contain "<a href=\"/subdir1\">subdir1</a>" list_response
'

test_expect_success "Valid parent directory path in directory listing at {cid}.ipfs.localhost/sub/dir" '
  curl -s "http://${DIR_CID}.ipfs.localhost:$GWAY_PORT/subdir1/subdir2/" > list_response &&
  test_should_contain "<a href=\"/subdir1/subdir2/./..\">..</a>" list_response &&
  test_should_contain "<a href=\"/subdir1/subdir2/bar\">bar</a>" list_response
'
# TODO make "Index of /" show full content path, ex: "index of /ipfs/<cid>"
# test_should_contain "Index of /ipfs/${DIR_CID}" list_response &&

test_expect_success "Request for deep path resource at {cid}.ipfs.localhost/sub/dir/file" '
  curl -s "http://${DIR_CID}.ipfs.localhost:$GWAY_PORT/subdir1/subdir2/bar" > list_response &&
  test_should_contain "subdir2-bar" list_response
'

# *.ipns.localhost


# switch to offline daemon to use local IPNS table
#test_kill_ipfs_daemon
#test_launch_ipfs_daemon --offline

# <libp2p-key>.ipns.localhost

test_localhost_gateway_response_should_contain \
  "Request for {CIDv1-libp2p-key}.ipns.localhost returns expected payload" \
  "http://${IPNS_IDv1}.ipns.localhost:$GWAY_PORT" \
  "hello"

test_localhost_gateway_response_should_contain \
  "Request for {CIDv1-dag-pb}.ipns.localhost redirects to CID with libp2p-key multicodec" \
  "http://${IPNS_IDv1_DAGPB}.ipns.localhost:$GWAY_PORT" \
  "Location: http://${IPNS_IDv1}.ipns.localhost:$GWAY_PORT/"

# TODO: <dnslink-fqdn>.ipns.localhost
# - Opening <dnslink-fqdn>.ipns.localhost DNSLink (Host header) (eg. http://en.wikipedia-on-ipfs.org?)

# TODO: this needs to be instant
#test_expect_success "Request for localhost/ipns/{fqdn} redirects to DNSLink in subdomain" '
#  DOCS_CID=$(ipfs name resolve -r docs.ipfs.io | cut -d"/" -f3) &&
#  echo $DOCS_CID &&
#  curl "http://docs.ipfs.io.ipns.localhost:$GWAY_PORT" > dnslink_response &&
#  curl "$GWAY_ADDR/ipfs/$DOCS_CID" > docs_cid_expected &&
#  test_cmp docs_cid_expected dnslink_response
#'


## ============================================================================
## Test subdomain-based requests with a custom hostname config
## (origin per content root at http://*.example.com)
## ============================================================================

# set explicit subdomain gateway config for the hostname
ipfs config --json Gateway.PublicGateways '{"example.com": { "UseSubdomains": true, "Paths": ["/ipfs", "/ipns"] }}'
# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon --offline

# example.com/ip(f|n)s/*
# =============================================================================

# path requests to the root hostname should redirect
# to a subdomain URL with proper origin isolation

test_hostname_gateway_response_should_contain \
  "Request for example.com/ipfs/{CIDv1} produces redirect to {CIDv1}.ipfs.example.com" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "Location: http://$CIDv1.ipfs.example.com/"

test_hostname_gateway_response_should_contain \
  "Request for example.com/ipfs/{CIDv0} produces redirect to {CIDv1}.ipfs.example.com" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv0" \
  "Location: http://${CIDv0to1}.ipfs.example.com/"

# example.com/ipns/<libp2p-key>

test_hostname_gateway_response_should_contain \
  "Request for example.com/ipns/{CIDv0} redirects to CIDv1 with libp2p-key multicodec in subdomain" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_IDv0"
  "Location: http://${IPNS_IDv1}.ipns.example.com/"

# example.com/ipns/<dnslink-fqdn>

test_hostname_gateway_response_should_contain \
  "Request for example.com/ipns/{fqdn} redirects to DNSLink in subdomain" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipns/en.wikipedia-on-ipfs.org/wiki" \
  "Location: http://en.wikipedia-on-ipfs.org.ipns.example.com/wiki"

# *.ipfs.example.com: subdomain requests made with custom FQDN in Host header

test_hostname_gateway_response_should_contain \
  "Request for {CID}.ipfs.example.com should return expected payload" \
  "${CIDv1}.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "hello"

test_hostname_gateway_response_should_contain \
  "Request for {CID}.ipfs.example.com/ipfs/{CID} should return HTTP 404" \
  "${CIDv1}.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "404 Not Found"

# {CID}.ipfs.example.com/sub/dir (Directory Listing)

test_expect_success "Valid file and directory paths in directory listing at {cid}.ipfs.example.com" '
  curl -s -H "Host: ${DIR_CID}.ipfs.example.com" http://127.0.0.1:$GWAY_PORT > list_response &&
  test_should_contain "<a href=\"/hello\">hello</a>" list_response &&
  test_should_contain "<a href=\"/subdir1\">subdir1</a>" list_response
'

test_expect_success "Valid parent directory path in directory listing at {cid}.ipfs.localhost/sub/dir" '
  curl -s -H "Host: ${DIR_CID}.ipfs.example.com" http://127.0.0.1:$GWAY_PORT/subdir1/subdir2/ > list_response &&
  test_should_contain "<a href=\"/subdir1/subdir2/./..\">..</a>" list_response &&
  test_should_contain "<a href=\"/subdir1/subdir2/bar\">bar</a>" list_response
'

test_expect_success "Request for deep path resource {cid}.ipfs.localhost/sub/dir/file" '
  curl -s -H "Host: ${DIR_CID}.ipfs.example.com" http://127.0.0.1:$GWAY_PORT/subdir1/subdir2/bar > list_response &&
  curl -s "http://${DIR_CID}.ipfs.localhost:$GWAY_PORT/subdir1/subdir2/bar" > list_response &&
  test_should_contain "subdir2-bar" list_response
'

# *.ipns.example.com
# ============================================================================

# <libp2p-key>.ipns.example.com

test_hostname_gateway_response_should_contain \
  "Request for {CIDv1-libp2p-key}.ipns.example.com returns expected payload" \
  "${IPNS_IDv1}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "hello"

test_hostname_gateway_response_should_contain \
  "Request for {CIDv1-dag-pb}.ipns.localhost redirects to CID with libp2p-key multicodec" \
  "${IPNS_IDv1_DAGPB}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "Location: http://${IPNS_IDv1}.ipns.example.com/"

# TODO: api.example.com/v0/id
#
# DNSLink requests (could be moved to separate test file)
#
# - set PublicGateway config for host with DNSLink, eg. docs.ipfs.io
#   - Paths: [] NoDNSLink: false
#     - confirm content-addressed requests return 404
#     - confirm the same payload is returned for / as for path at `ipfs dns docs.ipfs.io`
#   - Paths: [] NoDNSLink: true
#     - confirm both DNSLink and content-addressing return 404
#

# Disable selected Paths for the subdomain gateway hostname
# =============================================================================

# disable /ipns for the hostname by not whitelisting it
ipfs config --json Gateway.PublicGateways '{"example.com": { "UseSubdomains": true, "Paths": ["/ipfs"] }}'
# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon --offline

# refuse requests to Paths that were not explicitly whitelisted for the hostname
test_hostname_gateway_response_should_contain \
  "Request for *.ipns.example.com is refused due to Paths restricted to /ipfs" \
  "${IPNS_IDv1}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "403 Forbidden"


## ============================================================================
## Test path-based requests with a custom hostname config
## ============================================================================

# set explicit subdomain gateway config for the hostname
ipfs config --json Gateway.PublicGateways '{"example.com": { "UseSubdomains": false, "Paths": ["/ipfs"] }}'
# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon --offline

# example.com/ip(f|n)s/* smoke-tests
# =============================================================================

# confirm path gateway works for /ipfs
test_hostname_gateway_response_should_contain \
  "Request for example.com/ipfs/{CIDv1} returns expected payload" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "hello"

# refuse subdomain requests on path gateway
# (we don't want false sense of security)
test_hostname_gateway_response_should_contain \
  "Request for {CID}.ipfs.example.com/ipfs/{CID} should return HTTP 403 Forbidden" \
  "${CIDv1}.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "403 Forbidden"

# refuse requests to Paths that were not explicitly whitelisted for the hostname
test_hostname_gateway_response_should_contain \
  "Request for example.com/ipns/ is refused due to Paths restricted to /ipfs" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_IDv1" \
  "403 Forbidden"

# =============================================================================
test_kill_ipfs_daemon

test_done
