#!/usr/bin/env bash

test_description="Test HTTP Gateway _redirects support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

## ============================================================================
## Test _redirects file support
## ============================================================================

# Import test case
# Run `ipfs cat /ipfs/$REDIRECTS_DIR_CID/_redirects` to see sample _redirects file
test_expect_success "Add the _redirects file test directory" '
  ipfs dag import --pin-roots ../t0109-gateway-web-_redirects-data/redirects.car
'
CAR_ROOT_CID=QmQyqMY5vUBSbSxyitJqthgwZunCQjDVtNd8ggVCxzuPQ4

REDIRECTS_DIR_CID=$(ipfs resolve -r /ipfs/$CAR_ROOT_CID/examples | cut -d "/" -f3)
REDIRECTS_DIR_HOSTNAME="${REDIRECTS_DIR_CID}.ipfs.localhost:$GWAY_PORT"

# This test ensures _redirects is supported only on Web Gateways that use Host header (DNSLink, Subdomain)
test_expect_success "request for http://127.0.0.1:$GWAY_PORT/ipfs/$REDIRECTS_DIR_CID/301-redirect-one returns generic 404 (no custom 404 from _redirects since no origin isolation)" '
  curl -sD - "http://127.0.0.1:$GWAY_PORT/ipfs/$REDIRECTS_DIR_CID/301-redirect-one" > response &&
  test_should_contain "404 Not Found" response &&
  test_should_not_contain "my 404" response
'
test_kill_ipfs_daemon

# disable wildcard DNSLink gateway
# and enable it on specific DNSLink hostname
ipfs config --json Gateway.NoDNSLink true && \
ipfs config --json Gateway.PublicGateways '{
  "dnslink-enabled-on-fqdn.example.org": {
    "NoDNSLink": false,
    "UseSubdomains": false,
    "Paths": ["/ipfs"]
  },
  "dnslink-disabled-on-fqdn.example.com": {
    "NoDNSLink": true,
    "UseSubdomains": false,
    "Paths": []
  }
}' || exit 1

# DNSLink test requires a daemon in online mode with precached /ipns/ mapping
# REDIRECTS_DIR_CID=$(ipfs resolve -r /ipfs/$CAR_ROOT_CID/examples | cut -d "/" -f3)
DNSLINK_FQDN="dnslink-enabled-on-fqdn.example.org"
NO_DNSLINK_FQDN="dnslink-disabled-on-fqdn.example.com"
export IPFS_NS_MAP="$DNSLINK_FQDN:/ipfs/$REDIRECTS_DIR_CID"

# restart daemon to apply config changes
test_launch_ipfs_daemon

# make sure test setup is valid (fail if CoreAPI is unable to resolve)
test_expect_success "spoofed DNSLink record resolves in cli" "
  ipfs resolve /ipns/$DNSLINK_FQDN > result &&
  test_should_contain \"$REDIRECTS_DIR_CID\" result &&
  ipfs cat /ipns/$DNSLINK_FQDN/_redirects > result &&
  test_should_contain \"index.html\" result
"

test_expect_success "request for $NO_DNSLINK_FQDN/redirect-one does not redirect, since DNSLink is disabled" '
  curl -sD - --resolve $NO_DNSLINK_FQDN:$GWAY_PORT:127.0.0.1 "http://$NO_DNSLINK_FQDN:$GWAY_PORT/redirect-one" > response &&
  test_should_not_contain "one.html" response &&
  test_should_not_contain "301 Moved Permanently" response &&
  test_should_not_contain "Location:" response
'

test_kill_ipfs_daemon

test_done
