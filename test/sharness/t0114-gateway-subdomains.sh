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

# Helper that checks gateway response for specific hostname in Host header
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

test_expect_success "ipfs init" '
  export IPFS_PATH="$(pwd)/.ipfs" &&
  ipfs init --profile=test > /dev/null
'

test_launch_ipfs_daemon_without_network

# Import test case
# See the static fixtures in ./t0114-gateway-subdomains/
CID_VAL=hello
CIDv1=bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am
CIDv0=QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
CIDv0to1=bafybeiffndsajwhk3lwjewwdxqntmjm4b5wxaaanokonsggenkbw6slwk4
CIDv1_TOO_LONG=bafkrgqhhyivzstcz3hhswshfjgy6ertgmnqeleynhwt4dlfsthi4hn7zgh4uvlsb5xncykzapi3ocd4lzogukir6ksdy6wzrnz6ohnv4aglcs
DIR_CID=bafybeiht6dtwk3les7vqm6ibpvz6qpohidvlshsfyr7l5mpysdw2vmbbhe

RSA_KEY=QmVujd5Vb7moysJj8itnGufN7MEtPRCNHkKpNuA4onsRa3
RSA_IPNS_IDv0=QmVujd5Vb7moysJj8itnGufN7MEtPRCNHkKpNuA4onsRa3
RSA_IPNS_IDv1=k2k4r8m7xvggw5pxxk3abrkwyer625hg01hfyggrai7lk1m63fuihi7w
RSA_IPNS_IDv1_DAGPB=k2jmtxu61bnhrtj301lw7zizknztocdbeqhxgv76l2q9t36fn9jbzipo

ED25519_KEY=12D3KooWLQzUv2FHWGVPXTXSZpdHs7oHbXub2G5WC8Tx4NQhyd2d
ED25519_IPNS_IDv0=12D3KooWLQzUv2FHWGVPXTXSZpdHs7oHbXub2G5WC8Tx4NQhyd2d
ED25519_IPNS_IDv1=k51qzi5uqu5dk3v4rmjber23h16xnr23bsggmqqil9z2gduiis5se8dht36dam
ED25519_IPNS_IDv1_DAGPB=k50rm9yjlt0jey4fqg6wafvqprktgbkpgkqdg27tpqje6iimzxewnhvtin9hhq
IPNS_ED25519_B58MH=12D3KooWLQzUv2FHWGVPXTXSZpdHs7oHbXub2G5WC8Tx4NQhyd2d
IPNS_ED25519_B36CID=k51qzi5uqu5dk3v4rmjber23h16xnr23bsggmqqil9z2gduiis5se8dht36dam

test_expect_success "Add the test fixtures" '
  ipfs dag import --pin-roots ../t0114-gateway-subdomains/fixtures.car &&
  ipfs routing put --allow-offline /ipns/${RSA_KEY} ../t0114-gateway-subdomains/${RSA_KEY}.ipns-record &&
  ipfs routing put --allow-offline /ipns/${ED25519_KEY} ../t0114-gateway-subdomains/${ED25519_KEY}.ipns-record
'

# ensure we start with empty Gateway.PublicGateways
test_expect_success 'start daemon with empty config for Gateway.PublicGateways' '
  test_kill_ipfs_daemon &&
  ipfs config --json Gateway.PublicGateways "{}" &&
  test_launch_ipfs_daemon_without_network
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

# {CID}.ipfs.localhost/sub/dir (Directory Listing)
DIR_HOSTNAME="${DIR_CID}.ipfs.localhost:$GWAY_PORT"

# <dnslink-fqdn>.ipns.localhost

# api.localhost/api

# Note: we use DIR_CID so refs -r returns some CIDs for child nodes
test_localhost_gateway_response_should_contain \
  "request for api.localhost returns API response" \
  "http://api.localhost:$GWAY_PORT/api/v0/refs?arg=$DIR_CID&r=true" \
  "Ref"

## ============================================================================
## Test DNSLink inlining on HTTP gateways
## ============================================================================

# set explicit subdomain gateway config for the hostname
ipfs config --json Gateway.PublicGateways '{
  "localhost": {
    "UseSubdomains": true,
    "InlineDNSLink": true,
    "Paths": ["/ipfs", "/ipns", "/api"]
  },
  "example.com": {
    "UseSubdomains": true,
    "InlineDNSLink": true,
    "Paths": ["/ipfs", "/ipns", "/api"]
  }
}' || exit 1
# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon_without_network

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
test_launch_ipfs_daemon_without_network


# example.com/ip(f|n)s/*
# =============================================================================

# *.ipfs.example.com: subdomain requests made with custom FQDN in Host header

# {CID}.ipfs.example.com/sub/dir (Directory Listing)
DIR_FQDN="${DIR_CID}.ipfs.example.com"

# *.ipns.example.com
# ============================================================================

# <libp2p-key>.ipns.example.com

test_hostname_gateway_response_should_contain \
  "request for {CIDv1-libp2p-key}.ipns.example.com returns expected payload" \
  "${RSA_IPNS_IDv1}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "$CID_VAL"

test_hostname_gateway_response_should_contain \
  "request for {CIDv1-libp2p-key}.ipns.example.com returns expected payload" \
  "${ED25519_IPNS_IDv1}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "$CID_VAL"

test_hostname_gateway_response_should_contain \
  "hostname request for {CIDv1-dag-pb}.ipns.localhost redirects to CID with libp2p-key multicodec" \
  "${RSA_IPNS_IDv1_DAGPB}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "Location: http://${RSA_IPNS_IDv1}.ipns.example.com/"

test_hostname_gateway_response_should_contain \
  "hostname request for {CIDv1-dag-pb}.ipns.localhost redirects to CID with libp2p-key multicodec" \
  "${ED25519_IPNS_IDv1_DAGPB}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "Location: http://${ED25519_IPNS_IDv1}.ipns.example.com/"

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
test_launch_ipfs_daemon_without_network

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
test_launch_ipfs_daemon_without_network

# refuse requests to Paths that were not explicitly whitelisted for the hostname
test_hostname_gateway_response_should_contain \
  "request for *.ipns.example.com returns HTTP 404 Not Found when /ipns is not on Paths whitelist" \
  "${RSA_IPNS_IDv1}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "404 Not Found"

test_hostname_gateway_response_should_contain \
  "request for *.ipns.example.com returns HTTP 404 Not Found when /ipns is not on Paths whitelist" \
  "${ED25519_IPNS_IDv1}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "404 Not Found"

## ============================================================================
## Test path-based requests with a custom hostname config
## ============================================================================

# set explicit no-subdomain gateway config for the hostname
ipfs config --json Gateway.PublicGateways '{
  "example.com": {
    "UseSubdomains": false,
    "Paths": ["/ipfs"]
  }
}' || exit 1

# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon_without_network

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
  "http://127.0.0.1:$GWAY_PORT/ipns/$RSA_IPNS_IDv1" \
  "404 Not Found"

test_hostname_gateway_response_should_contain \
  "request for example.com/ipns/ returns HTTP 404 Not Found when /ipns is not on Paths whitelist" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipns/$ED25519_IPNS_IDv1" \
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
  "http://127.0.0.1:$GWAY_PORT/ipns/$RSA_IPNS_IDv0" \
  "404 Not Found"

test_hostname_gateway_response_should_contain \
  "request for {dnslink-fqdn}/ipns/{peerid} returns 404 when path is not whitelisted" \
  "$DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/ipns/$ED25519_IPNS_IDv0" \
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

## ============================================================================
## Test support for wildcards in gateway config
## ============================================================================

# set explicit subdomain gateway config for the hostnames
ipfs config --json Gateway.PublicGateways '{
  "*.example1.com": {
    "UseSubdomains": true,
    "Paths": ["/ipfs"]
  },
  "*.*.example2.com": {
    "UseSubdomains": true,
    "Paths": ["/ipfs"]
  },
  "foo.*.example3.com": {
    "UseSubdomains": true,
    "Paths": ["/ipfs"]
  },
  "foo.bar-*-boo.example4.com": {
    "UseSubdomains": true,
    "Paths": ["/ipfs"]
  }
}' || exit 1
# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon_without_network

# *.example1.com

test_hostname_gateway_response_should_contain \
  "request for foo.example1.com/ipfs/{CIDv1} produces redirect to {CIDv1}.ipfs.foo.example1.com" \
  "foo.example1.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "Location: http://$CIDv1.ipfs.foo.example1.com/"

test_hostname_gateway_response_should_contain \
  "request for {CID}.ipfs.foo.example1.com should return expected payload" \
  "${CIDv1}.ipfs.foo.example1.com" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "$CID_VAL"

# *.*.example2.com

test_hostname_gateway_response_should_contain \
  "request for foo.bar.example2.com/ipfs/{CIDv1} produces redirect to {CIDv1}.ipfs.foo.bar.example2.com" \
  "foo.bar.example2.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "Location: http://$CIDv1.ipfs.foo.bar.example2.com/"

test_hostname_gateway_response_should_contain \
  "request for {CID}.ipfs.foo.bar.example2.com should return expected payload" \
  "${CIDv1}.ipfs.foo.bar.example2.com" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "$CID_VAL"

# foo.*.example3.com

test_hostname_gateway_response_should_contain \
  "request for foo.bar.example3.com/ipfs/{CIDv1} produces redirect to {CIDv1}.ipfs.foo.bar.example3.com" \
  "foo.bar.example3.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "Location: http://$CIDv1.ipfs.foo.bar.example3.com/"

test_hostname_gateway_response_should_contain \
  "request for {CID}.ipfs.foo.bar.example3.com should return expected payload" \
  "${CIDv1}.ipfs.foo.bar.example3.com" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "$CID_VAL"

# foo.bar-*-boo.example4.com

test_hostname_gateway_response_should_contain \
  "request for foo.bar-dev-boo.example4.com/ipfs/{CIDv1} produces redirect to {CIDv1}.ipfs.foo.bar-dev-boo.example4.com" \
  "foo.bar-dev-boo.example4.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "Location: http://$CIDv1.ipfs.foo.bar-dev-boo.example4.com/"

test_hostname_gateway_response_should_contain \
  "request for {CID}.ipfs.foo.bar-dev-boo.example4.com should return expected payload" \
  "${CIDv1}.ipfs.foo.bar-dev-boo.example4.com" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "$CID_VAL"

## ============================================================================
## Test support for overriding implicit defaults
## ============================================================================

# disable subdomain gateway at localhost by removing implicit config
ipfs config --json Gateway.PublicGateways '{
  "localhost": null
}' || exit 1

# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon_without_network

test_localhost_gateway_response_should_contain \
  "request for localhost/ipfs/{CID} stays on path when subdomain gw is explicitly disabled" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv1" \
  "$CID_VAL"

# =============================================================================
# ensure we end with empty Gateway.PublicGateways
ipfs config --json Gateway.PublicGateways '{}'
test_kill_ipfs_daemon

test_expect_success "clean up ipfs dir" '
  rm -rf "$IPFS_PATH"
'

test_done
