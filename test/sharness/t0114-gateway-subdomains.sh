#!/usr/bin/env bash
#
# Copyright (c) Protocol Labs

test_description="Test subdomain support on the HTTP gateway"


. lib/test-lib.sh

## ============================================================================
## Helpers specific to subdomain tests
## ============================================================================

# Helper that tests gateway response over direct HTTP
# and in all supported HTTP proxy modes
test_localhost_gateway_response_should_contain() {
  local label="$1"
  local expected="$3"

  # explicit "Host: $hostname" header to match browser behavior
  # and also make tests independent from DNS
  local host=$(echo $2 | cut -d'/' -f3 | cut -d':' -f1)
  local hostname=$(echo $2 | cut -d'/' -f3 | cut -d':' -f1,2)

  # Proxy is the same as HTTP Gateway, we use raw IP and port to be sure
  local proxy="http://127.0.0.1:$GWAY_PORT"

  # Create a raw URL version with IP to ensure hostname from Host header is used
  # (removes false-positives, Host header is used for passing hostname already)
  local url="$2"
  local rawurl=$(echo "$url" | sed "s/$hostname/127.0.0.1:$GWAY_PORT/")

  #echo "hostname:   $hostname"
  #echo "url before: $url"
  #echo "url after:  $rawurl"

  # regular HTTP request
  # (hostname in Host header, raw IP in URL)
  test_expect_success "$label (direct HTTP)" "
    curl -H \"Host: $hostname\" -sD - \"$rawurl\" > response &&
    test_should_contain \"$expected\" response
  "

  # HTTP proxy
  # (hostname is passed via URL)
  # Note: proxy client should not care, but curl does DNS lookup
  # for some reason anyway, so we pass static DNS mapping
  test_expect_success "$label (HTTP proxy)" "
    curl -x $proxy --resolve $hostname:127.0.0.1 -sD - \"$url\" > response &&
    test_should_contain \"$expected\" response
  "

  # HTTP proxy 1.0
  # (repeating proxy test with older spec, just to be sure)
  test_expect_success "$label (HTTP proxy 1.0)" "
    curl --proxy1.0 $proxy --resolve $hostname:127.0.0.1 -sD - \"$url\" > response &&
    test_should_contain \"$expected\" response
  "

  # HTTP proxy tunneling (CONNECT)
  # https://tools.ietf.org/html/rfc7231#section-4.3.6
  # In HTTP/1.x, the pseudo-method CONNECT
  # can be used to convert an HTTP connection into a tunnel to a remote host
  test_expect_success "$label (HTTP proxy tunneling)" "
    curl --proxytunnel -x $proxy -H \"Host: $hostname\" -sD - \"$rawurl\" > response &&
    test_should_contain \"$expected\" response
  "
}

# Helper that checks gateway resonse for specific hostname in Host header
test_hostname_gateway_response_should_contain() {
  local label="$1"
  local hostname="$2"
  local url="$3"
  local rawurl=$(echo "$url" | sed "s/$hostname/127.0.0.1:$GWAY_PORT/")
  local expected="$4"
  test_expect_success "$label" "
    curl -H \"Host: $hostname\" -sD - \"$rawurl\" > response &&
    test_should_contain \"$expected\" response
  "
}

## ============================================================================
## Start IPFS Node and prepare test CIDs
## ============================================================================

test_init_ipfs
test_launch_ipfs_daemon --offline

# CIDv0to1 is necessary because raw-leaves are enabled by default during
# "ipfs add" with CIDv1 and disabled with CIDv0
test_expect_success "Add test text file" '
  CID_VAL="hello"
  CIDv1=$(echo $CID_VAL | ipfs add --cid-version 1 -Q)
  CIDv0=$(echo $CID_VAL | ipfs add --cid-version 0 -Q)
  CIDv0to1=$(echo "$CIDv0" | ipfs cid base32)
'

test_expect_success "Add the test directory" '
  mkdir -p testdirlisting/subdir1/subdir2 &&
  echo "hello" > testdirlisting/hello &&
  echo "subdir2-bar" > testdirlisting/subdir1/subdir2/bar &&
  mkdir -p testdirlisting/api &&
  mkdir -p testdirlisting/ipfs &&
  echo "I am a txt file" > testdirlisting/api/file.txt &&
  echo "I am a txt file" > testdirlisting/ipfs/file.txt &&
  DIR_CID=$(ipfs add -Qr --cid-version 1 testdirlisting)
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


# ensure we start with empty Gateway.PublicGateways
test_expect_success 'start daemon with empty config for Gateway.PublicGateways' '
  test_kill_ipfs_daemon &&
  ipfs config --json Gateway.PublicGateways "{}" &&
  test_launch_ipfs_daemon --offline
'

## ============================================================================
## Test path-based requests to a local gateway with default config
## (forced redirects to http://*.localhost)
## ============================================================================

# /ipfs/<cid>

# IP remains old school path-based gateway

test_localhost_gateway_response_should_contain \
  "request for 127.0.0.1/ipfs/{CID} stays on path" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "$CID_VAL"

# 'localhost' hostname is used for subdomains, and should not return
#  payload directly, but redirect to URL with proper origin isolation

test_localhost_gateway_response_should_contain \
  "request for localhost/ipfs/{CIDv1} returns status code HTTP 301" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv1" \
  "301 Moved Permanently"

test_localhost_gateway_response_should_contain \
  "request for localhost/ipfs/{CIDv1} returns Location HTTP header for subdomain redirect in browsers" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv1" \
  "Location: http://$CIDv1.ipfs.localhost:$GWAY_PORT/"

# Responses to the root domain of subdomain gateway hostname should Clear-Site-Data
# https://github.com/ipfs/go-ipfs/issues/6975#issuecomment-597472477
test_localhost_gateway_response_should_contain \
  "request for localhost/ipfs/{CIDv1} returns Clear-Site-Data header to purge Origin cookies and storage" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv1" \
  'Clear-Site-Data: \"cookies\", \"storage\"'

# We return body with HTTP 301 so existing cli scripts that use path-based
# gateway do not break (curl doesn't auto-redirect without passing -L; wget
# does not span across hostnames by default)
# Context: https://github.com/ipfs/go-ipfs/issues/6975
test_localhost_gateway_response_should_contain \
  "request for localhost/ipfs/{CIDv1} includes valid payload in body for CLI tools like curl" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv1" \
  "$CID_VAL"

test_localhost_gateway_response_should_contain \
  "request for localhost/ipfs/{CIDv0} redirects to CIDv1 representation in subdomain" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv0" \
  "Location: http://${CIDv0to1}.ipfs.localhost:$GWAY_PORT/"

# /ipns/<libp2p-key>

test_localhost_gateway_response_should_contain \
  "request for localhost/ipns/{CIDv0} redirects to CIDv1 with libp2p-key multicodec in subdomain" \
  "http://localhost:$GWAY_PORT/ipns/$IPNS_IDv0" \
  "Location: http://${IPNS_IDv1}.ipns.localhost:$GWAY_PORT/"

# /ipns/<dnslink-fqdn>

test_localhost_gateway_response_should_contain \
  "request for localhost/ipns/{fqdn} redirects to DNSLink in subdomain" \
  "http://localhost:$GWAY_PORT/ipns/en.wikipedia-on-ipfs.org/wiki" \
  "Location: http://en.wikipedia-on-ipfs.org.ipns.localhost:$GWAY_PORT/wiki"

# API on localhost subdomain gateway

# /api/v0 present on the root hostname
test_localhost_gateway_response_should_contain \
  "request for localhost/api" \
  "http://localhost:$GWAY_PORT/api/v0/refs?arg=${DIR_CID}&r=true" \
  "Ref"

# /api/v0 not mounted on content root subdomains
test_localhost_gateway_response_should_contain \
  "request for {cid}.ipfs.localhost/api returns data if present on the content root" \
  "http://${DIR_CID}.ipfs.localhost:$GWAY_PORT/api/file.txt" \
  "I am a txt file"

test_localhost_gateway_response_should_contain \
  "request for {cid}.ipfs.localhost/api/v0/refs returns 404" \
  "http://${DIR_CID}.ipfs.localhost:$GWAY_PORT/api/v0/refs?arg=${DIR_CID}&r=true" \
  "404 Not Found"

## ============================================================================
## Test subdomain-based requests to a local gateway with default config
## (origin per content root at http://*.localhost)
## ============================================================================

# {CID}.ipfs.localhost

test_localhost_gateway_response_should_contain \
  "request for {CID}.ipfs.localhost should return expected payload" \
  "http://${CIDv1}.ipfs.localhost:$GWAY_PORT" \
  "$CID_VAL"

# ensure /ipfs/ namespace is not mounted on subdomain
test_localhost_gateway_response_should_contain \
  "request for {CID}.ipfs.localhost/ipfs/{CID} should return HTTP 404" \
  "http://${CIDv1}.ipfs.localhost:$GWAY_PORT/ipfs/$CIDv1" \
  "404 Not Found"

# ensure requests to /ipfs/* are not blocked, if content root has such subdirectory
test_localhost_gateway_response_should_contain \
  "request for {CID}.ipfs.localhost/ipfs/file.txt should return data from a file in CID content root" \
  "http://${DIR_CID}.ipfs.localhost:$GWAY_PORT/ipfs/file.txt" \
  "I am a txt file"

# {CID}.ipfs.localhost/sub/dir (Directory Listing)
DIR_HOSTNAME="${DIR_CID}.ipfs.localhost:$GWAY_PORT"

test_expect_success "valid file and subdirectory paths in directory listing at {cid}.ipfs.localhost" '
  curl -s --resolve $DIR_HOSTNAME:127.0.0.1 "http://$DIR_HOSTNAME" > list_response &&
  test_should_contain "<a href=\"/hello\">hello</a>" list_response &&
  test_should_contain "<a href=\"/subdir1\">subdir1</a>" list_response
'

test_expect_success "valid parent directory path in directory listing at {cid}.ipfs.localhost/sub/dir" '
  curl -s --resolve $DIR_HOSTNAME:127.0.0.1 "http://$DIR_HOSTNAME/subdir1/subdir2/" > list_response &&
  test_should_contain "<a href=\"/subdir1/subdir2/./..\">..</a>" list_response &&
  test_should_contain "<a href=\"/subdir1/subdir2/bar\">bar</a>" list_response
'

test_expect_success "request for deep path resource at {cid}.ipfs.localhost/sub/dir/file" '
  curl -s --resolve $DIR_HOSTNAME:127.0.0.1 "http://$DIR_HOSTNAME/subdir1/subdir2/bar" > list_response &&
  test_should_contain "subdir2-bar" list_response
'

# *.ipns.localhost

# <libp2p-key>.ipns.localhost

test_localhost_gateway_response_should_contain \
  "request for {CIDv1-libp2p-key}.ipns.localhost returns expected payload" \
  "http://${IPNS_IDv1}.ipns.localhost:$GWAY_PORT" \
  "$CID_VAL"

test_localhost_gateway_response_should_contain \
  "request for {CIDv1-dag-pb}.ipns.localhost redirects to CID with libp2p-key multicodec" \
  "http://${IPNS_IDv1_DAGPB}.ipns.localhost:$GWAY_PORT" \
  "Location: http://${IPNS_IDv1}.ipns.localhost:$GWAY_PORT/"

# <dnslink-fqdn>.ipns.localhost

# DNSLink test requires a daemon in online mode with precached /ipns/ mapping
test_kill_ipfs_daemon
DNSLINK_FQDN="dnslink-test.example.com"
export IPFS_NS_MAP="$DNSLINK_FQDN:/ipfs/$CIDv1"
test_launch_ipfs_daemon

test_localhost_gateway_response_should_contain \
  "request for {dnslink}.ipns.localhost returns expected payload" \
  "http://$DNSLINK_FQDN.ipns.localhost:$GWAY_PORT" \
  "$CID_VAL"

# api.localhost/api

# Note: we use DIR_CID so refs -r returns some CIDs for child nodes
test_localhost_gateway_response_should_contain \
  "request for api.localhost returns API response" \
  "http://api.localhost:$GWAY_PORT/api/v0/refs?arg=$DIR_CID&r=true" \
  "Ref"

## ============================================================================
## Test subdomain-based requests with a custom hostname config
## (origin per content root at http://*.example.com)
## ============================================================================

# set explicit subdomain gateway config for the hostname
ipfs config --json Gateway.PublicGateways '{
  "example.com": {
    "UseSubdomains": true,
    "Paths": ["/ipfs", "/ipns", "/api"]
  }
}' || exit 1
# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon --offline


# example.com/ip(f|n)s/*
# =============================================================================

# path requests to the root hostname should redirect
# to a subdomain URL with proper origin isolation

test_hostname_gateway_response_should_contain \
  "request for example.com/ipfs/{CIDv1} produces redirect to {CIDv1}.ipfs.example.com" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "Location: http://$CIDv1.ipfs.example.com/"

# error message should include original CID
# (and it should be case-sensitive, as we can't assume everyone uses base32)
test_hostname_gateway_response_should_contain \
  "request for example.com/ipfs/{InvalidCID} produces useful error before redirect" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/QmInvalidCID" \
  'invalid path \"/ipfs/QmInvalidCID\"'

test_hostname_gateway_response_should_contain \
  "request for example.com/ipfs/{CIDv0} produces redirect to {CIDv1}.ipfs.example.com" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv0" \
  "Location: http://${CIDv0to1}.ipfs.example.com/"

# Support X-Forwarded-Proto
test_expect_success "request for http://example.com/ipfs/{CID} with X-Forwarded-Proto: https produces redirect to HTTPS URL" "
  curl -H \"X-Forwarded-Proto: https\" -H \"Host: example.com\" -sD - \"http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1\" > response &&
  test_should_contain \"Location: https://$CIDv1.ipfs.example.com/\" response
"



# example.com/ipns/<libp2p-key>

test_hostname_gateway_response_should_contain \
  "request for example.com/ipns/{CIDv0} redirects to CIDv1 with libp2p-key multicodec in subdomain" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_IDv0" \
  "Location: http://${IPNS_IDv1}.ipns.example.com/"

# example.com/ipns/<dnslink-fqdn>

test_hostname_gateway_response_should_contain \
  "request for example.com/ipns/{fqdn} redirects to DNSLink in subdomain" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipns/en.wikipedia-on-ipfs.org/wiki" \
  "Location: http://en.wikipedia-on-ipfs.org.ipns.example.com/wiki"

# *.ipfs.example.com: subdomain requests made with custom FQDN in Host header

test_hostname_gateway_response_should_contain \
  "request for {CID}.ipfs.example.com should return expected payload" \
  "${CIDv1}.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "$CID_VAL"

test_hostname_gateway_response_should_contain \
  "request for {CID}.ipfs.example.com/ipfs/{CID} should return HTTP 404" \
  "${CIDv1}.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "404 Not Found"

# {CID}.ipfs.example.com/sub/dir (Directory Listing)
DIR_FQDN="${DIR_CID}.ipfs.example.com"

test_expect_success "valid file and directory paths in directory listing at {cid}.ipfs.example.com" '
  curl -s -H "Host: $DIR_FQDN" http://127.0.0.1:$GWAY_PORT > list_response &&
  test_should_contain "<a href=\"/hello\">hello</a>" list_response &&
  test_should_contain "<a href=\"/subdir1\">subdir1</a>" list_response
'

test_expect_success "valid parent directory path in directory listing at {cid}.ipfs.example.com/sub/dir" '
  curl -s -H "Host: $DIR_FQDN" http://127.0.0.1:$GWAY_PORT/subdir1/subdir2/ > list_response &&
  test_should_contain "<a href=\"/subdir1/subdir2/./..\">..</a>" list_response &&
  test_should_contain "<a href=\"/subdir1/subdir2/bar\">bar</a>" list_response
'

test_expect_success "request for deep path resource {cid}.ipfs.example.com/sub/dir/file" '
  curl -s -H "Host: $DIR_FQDN" http://127.0.0.1:$GWAY_PORT/subdir1/subdir2/bar > list_response &&
  test_should_contain "subdir2-bar" list_response
'

# *.ipns.example.com
# ============================================================================

# <libp2p-key>.ipns.example.com

test_hostname_gateway_response_should_contain \
  "request for {CIDv1-libp2p-key}.ipns.example.com returns expected payload" \
  "${IPNS_IDv1}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "$CID_VAL"

test_hostname_gateway_response_should_contain \
  "request for {CIDv1-dag-pb}.ipns.localhost redirects to CID with libp2p-key multicodec" \
  "${IPNS_IDv1_DAGPB}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "Location: http://${IPNS_IDv1}.ipns.example.com/"

# API on subdomain gateway example.com
# ============================================================================

# present at the root domain
test_hostname_gateway_response_should_contain \
  "request for example.com/api/v0/refs returns expected payload when /api is on Paths whitelist" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/api/v0/refs?arg=${DIR_CID}&r=true" \
  "Ref"

# not mounted on content root subdomains
test_hostname_gateway_response_should_contain \
  "request for {cid}.ipfs.example.com/api returns data if present on the content root" \
  "$DIR_CID.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/api/file.txt" \
  "I am a txt file"

test_hostname_gateway_response_should_contain \
  "request for {cid}.ipfs.example.com/api/v0/refs returns 404" \
  "$CIDv1.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/api/v0/refs?arg=${DIR_CID}&r=true" \
  "404 Not Found"

# disable /api on example.com
ipfs config --json Gateway.PublicGateways '{
  "example.com": {
    "UseSubdomains": true,
    "Paths": ["/ipfs", "/ipns"]
  }
}' || exit 1
# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon --offline

# not mounted at the root domain
test_hostname_gateway_response_should_contain \
  "request for example.com/api/v0/refs returns 404 if /api not on Paths whitelist" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/api/v0/refs?arg=${DIR_CID}&r=true" \
  "404 Not Found"

# not mounted on content root subdomains
test_hostname_gateway_response_should_contain \
  "request for {cid}.ipfs.example.com/api returns data if present on the content root" \
  "$DIR_CID.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/api/file.txt" \
  "I am a txt file"

# DNSLink: <dnslink-fqdn>.ipns.example.com
# (not really useful outside of localhost, as setting TLS for more than one
# level of wildcard is a pain, but we support it if someone really wants it)
# ============================================================================

# DNSLink test requires a daemon in online mode with precached /ipns/ mapping
test_kill_ipfs_daemon
DNSLINK_FQDN="dnslink-subdomain-gw-test.example.org"
export IPFS_NS_MAP="$DNSLINK_FQDN:/ipfs/$CIDv1"
test_launch_ipfs_daemon

test_hostname_gateway_response_should_contain \
  "request for {dnslink}.ipns.example.com returns expected payload" \
  "$DNSLINK_FQDN.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "$CID_VAL"

# Disable selected Paths for the subdomain gateway hostname
# =============================================================================

# disable /ipns for the hostname by not whitelisting it
ipfs config --json Gateway.PublicGateways '{
  "example.com": {
    "UseSubdomains": true,
    "Paths": ["/ipfs"]
  }
}' || exit 1
# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon --offline

# refuse requests to Paths that were not explicitly whitelisted for the hostname
test_hostname_gateway_response_should_contain \
  "request for *.ipns.example.com returns HTTP 404 Not Found when /ipns is not on Paths whitelist" \
  "${IPNS_IDv1}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "404 Not Found"


## ============================================================================
## Test path-based requests with a custom hostname config
## ============================================================================

# set explicit subdomain gateway config for the hostname
ipfs config --json Gateway.PublicGateways '{
  "example.com": {
    "UseSubdomains": false,
    "Paths": ["/ipfs"]
  }
}' || exit 1

# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon --offline

# example.com/ip(f|n)s/* smoke-tests
# =============================================================================

# confirm path gateway works for /ipfs
test_hostname_gateway_response_should_contain \
  "request for example.com/ipfs/{CIDv1} returns expected payload" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "$CID_VAL"

# refuse subdomain requests on path gateway
# (we don't want false sense of security)
test_hostname_gateway_response_should_contain \
  "request for {CID}.ipfs.example.com/ipfs/{CID} should return HTTP 404 Not Found" \
  "${CIDv1}.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "404 Not Found"

# refuse requests to Paths that were not explicitly whitelisted for the hostname
test_hostname_gateway_response_should_contain \
  "request for example.com/ipns/ returns HTTP 404 Not Found when /ipns is not on Paths whitelist" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_IDv1" \
  "404 Not Found"

## ============================================================================
## Test DNSLink requests with a custom PublicGateway (hostname config)
## (DNSLink site at http://dnslink-test.example.com)
## ============================================================================

test_kill_ipfs_daemon

# disable wildcard DNSLink gateway
# and enable it on specific NSLink hostname
ipfs config --json Gateway.NoDNSLink true && \
ipfs config --json Gateway.PublicGateways '{
  "dnslink-enabled-on-fqdn.example.org": {
    "NoDNSLink": false,
    "UseSubdomains": false,
    "Paths": ["/ipfs"]
  },
  "only-dnslink-enabled-on-fqdn.example.org": {
    "NoDNSLink": false,
    "UseSubdomains": false,
    "Paths": []
  },
  "dnslink-disabled-on-fqdn.example.com": {
    "NoDNSLink": true,
    "UseSubdomains": false,
    "Paths": []
  }
}' || exit 1

# DNSLink test requires a daemon in online mode with precached /ipns/ mapping
DNSLINK_FQDN="dnslink-enabled-on-fqdn.example.org"
ONLY_DNSLINK_FQDN="only-dnslink-enabled-on-fqdn.example.org"
NO_DNSLINK_FQDN="dnslink-disabled-on-fqdn.example.com"
export IPFS_NS_MAP="$DNSLINK_FQDN:/ipfs/$CIDv1,$ONLY_DNSLINK_FQDN:/ipfs/$DIR_CID"

# restart daemon to apply config changes
test_launch_ipfs_daemon

# make sure test setup is valid (fail if CoreAPI is unable to resolve)
test_expect_success "spoofed DNSLink record resolves in cli" "
  ipfs resolve /ipns/$DNSLINK_FQDN > result &&
  test_should_contain \"$CIDv1\" result &&
  ipfs cat /ipns/$DNSLINK_FQDN > result &&
  test_should_contain \"$CID_VAL\" result
"

# DNSLink enabled

test_hostname_gateway_response_should_contain \
  "request for http://{dnslink-fqdn}/ PublicGateway returns expected payload" \
  "$DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "$CID_VAL"

test_hostname_gateway_response_should_contain \
  "request for {dnslink-fqdn}/ipfs/{cid} returns expected payload when /ipfs is on Paths whitelist" \
  "$DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "$CID_VAL"

# Test for a fun edge case: DNSLink-only gateway without  /ipfs/ namespace
# mounted, and with subdirectory named "ipfs" ¯\_(ツ)_/¯
test_hostname_gateway_response_should_contain \
  "request for {dnslink-fqdn}/ipfs/file.txt returns data from content root when /ipfs in not on Paths whitelist" \
  "$ONLY_DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/file.txt" \
  "I am a txt file"

test_hostname_gateway_response_should_contain \
  "request for {dnslink-fqdn}/ipns/{peerid} returns 404 when path is not whitelisted" \
  "$DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_IDv0" \
  "404 Not Found"

# DNSLink disabled

test_hostname_gateway_response_should_contain \
  "request for http://{dnslink-fqdn}/ returns 404 when NoDNSLink=true" \
  "$NO_DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "404 Not Found"

test_hostname_gateway_response_should_contain \
  "request for {dnslink-fqdn}/ipfs/{cid} returns 404 when path is not whitelisted" \
  "$NO_DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv0" \
  "404 Not Found"


## ============================================================================
## Test wildcard DNSLink (any hostname, with default config)
## ============================================================================

test_kill_ipfs_daemon

# enable wildcard DNSLink gateway (any value in Host header)
# and remove custom PublicGateways
ipfs config --json Gateway.NoDNSLink false && \
ipfs config --json Gateway.PublicGateways '{}' || exit 1

# DNSLink test requires a daemon in online mode with precached /ipns/ mapping
DNSLINK_FQDN="wildcard-dnslink-not-in-config.example.com"
export IPFS_NS_MAP="$DNSLINK_FQDN:/ipfs/$CIDv1"

# restart daemon to apply config changes
test_launch_ipfs_daemon

# make sure test setup is valid (fail if CoreAPI is unable to resolve)
test_expect_success "spoofed DNSLink record resolves in cli" "
  ipfs resolve /ipns/$DNSLINK_FQDN > result &&
  test_should_contain \"$CIDv1\" result &&
  ipfs cat /ipns/$DNSLINK_FQDN > result &&
  test_should_contain \"$CID_VAL\" result
"

# gateway test
test_hostname_gateway_response_should_contain \
  "request for http://{dnslink-fqdn}/ (wildcard) returns expected payload" \
  "$DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "$CID_VAL"

# =============================================================================
# ensure we end with empty Gateway.PublicGateways
ipfs config --json Gateway.PublicGateways '{}'
test_kill_ipfs_daemon

test_done
