#!/usr/bin/env bash
#
# Copyright (c) Protocol Labs

test_description="Test subdomain support on the HTTP gateway"

. lib/test-lib.sh

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

test_gateway_response_should_contain \
  "Request for 127.0.0.1/ipfs/{CID} stays on path" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "hello"

# 'localhost' hostname is used for subdomains, and should not return
#  payload directly, but redirect to URL with proper origin isolation

test_gateway_response_should_contain \
  "Request for localhost/ipfs/{CIDv1} redirects to subdomain" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv1" \
  "Location: http://$CIDv1.ipfs.localhost:$GWAY_PORT/"

test_gateway_response_should_contain \
  "Request for localhost/ipfs/{CIDv0} redirects to CIDv1 representation in subdomain" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv0" \
  "Location: http://${CIDv0to1}.ipfs.localhost:$GWAY_PORT/"

# /ipns/<libp2p-key>

test_gateway_response_should_contain \
  "Request for localhost/ipns/{CIDv0} redirects to CIDv1 with libp2p-key multicodec in subdomain" \
  "http://localhost:$GWAY_PORT/ipns/$IPNS_IDv0"
  "Location: http://${IPNS_IDv1}.ipns.localhost:$GWAY_PORT/"

# /ipns/<dnslink-fqdn>

test_gateway_response_should_contain \
  "Request for localhost/ipns/{fqdn} redirects to DNSLink in subdomain" \
  "http://localhost:$GWAY_PORT/ipns/en.wikipedia-on-ipfs.org/wiki" \
  "Location: http://en.wikipedia-on-ipfs.org.ipns.localhost:$GWAY_PORT/wiki"

## ============================================================================
## Test subdomain-based requests to a local gateway with default config
## (origin per content root at http://*.localhost)
## ============================================================================

# {CID}.ipfs.localhost

test_gateway_response_should_contain \
  "Request for {CID}.ipfs.localhost should return expected payload" \
  "http://${CIDv1}.ipfs.localhost:$GWAY_PORT" \
  "hello"

test_gateway_response_should_contain \
  "Request for {CID}.ipfs.localhost/ipfs/{CID} should return HTTP 404" \
  "http://${CIDv1}.ipfs.localhost:$GWAY_PORT/ipfs/$CIDv1" \
  "404 Not Found"

# {CID}.ipfs.localhost/sub/dir

test_expect_success "Add the test directory" '
  mkdir -p testdirlisting/subdir1/subdir2 &&
  echo "hello" > testdirlisting/hello &&
  echo "subdir2-bar" > testdirlisting/subdir1/subdir2/bar &&
  DIR_CID=$(ipfs add -Qr --cid-version 1 testdirlisting)
'

test_expect_success "Test the directory listing at {cid}.ipfs.localhost" '
  curl -s "http://${DIR_CID}.ipfs.localhost:$GWAY_PORT" > list_response &&
  test_should_contain "<a href=\"/hello\">hello</a>" list_response &&
  test_should_contain "<a href=\"/subdir1\">subdir1</a>" list_response
'

test_expect_success "Test the directory listing at {cid}.ipfs.localhost/sub/dir" '
  curl -s "http://${DIR_CID}.ipfs.localhost:$GWAY_PORT/subdir1/subdir2/" > list_response &&
  test_should_contain "<a href=\"/subdir1/subdir2/./..\">..</a>" list_response &&
  test_should_contain "<a href=\"/subdir1/subdir2/bar\">bar</a>" list_response
'
# TODO make "Index of /" show full content path, ex: "index of /ipfs/<cid>"
# test_should_contain "Index of /ipfs/${DIR_CID}" list_response &&

test_expect_success "Test subdirectory response at {cid}.ipfs.localhost/sub/dir/file" '
  curl -s "http://${DIR_CID}.ipfs.localhost:$GWAY_PORT/subdir1/subdir2/bar" > list_response &&
  test_should_contain "subdir2-bar" list_response
'

# *.ipns.localhost


# switch to offline daemon to use local IPNS table
#test_kill_ipfs_daemon
#test_launch_ipfs_daemon --offline

# <libp2p-key>.ipns.localhost

test_gateway_response_should_contain \
  "Request for {CIDv1-libp2p-key}.ipns.localhost return expected payload" \
  "http://${IPNS_IDv1}.ipns.localhost:$GWAY_PORT" \
  "hello"

test_gateway_response_should_contain \
  "Request for {CIDv1-dag-pb}.ipns.localhost redirects to CID with libp2p-key multicodec" \
  "http://${IPNS_IDv1_DAGPB}.ipns.localhost:$GWAY_PORT" \
  "Location: http://${IPNS_IDv1}.ipns.localhost:$GWAY_PORT/"

# <dnslink-fqdn>.ipns.localhost

# TODO: this needs to be instant
#test_expect_success "Request for localhost/ipns/{fqdn} redirects to DNSLink in subdomain" '
#  DOCS_CID=$(ipfs name resolve -r docs.ipfs.io | cut -d"/" -f3) &&
#  echo $DOCS_CID &&
#  curl "http://docs.ipfs.io.ipns.localhost:$GWAY_PORT" > dnslink_response &&
#  curl "$GWAY_ADDR/ipfs/$DOCS_CID" > docs_cid_expected &&
#  test_cmp docs_cid_expected dnslink_response
#'

# TODO: Write tests for DNSLink to a separate file
#
#   -  <dnslink-fqdn>.ipns.localhost DNSLink (Host header)  http://en.wikipedia-on-ipfs.org resolves
#
# subdomain requests with custom FQDN
#
# - set PublicGateways config for example.com
# *.ipfs.example.com (entry in PublicGateways + request via  curl -H "Host: {cid}.ipfs.example.com"
#   - with Paths: [/ipfs, ipns], NoDNSLink: false
#     - redirect example.com/ip* paths to subdomain
#     - confirm text file contents
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

test_kill_ipfs_daemon

test_done
