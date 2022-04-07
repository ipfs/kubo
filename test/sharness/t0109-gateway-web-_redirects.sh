#!/usr/bin/env bash

test_description="Test HTTP Gateway _redirects support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

## ============================================================================
## Test _redirects file support
## ============================================================================

# Directory tree crafted to test _redirects file support
test_expect_success "Add the _redirects file test directory" '
  mkdir -p testredirect/ &&
  echo "my index" > testredirect/index.html &&
  echo "my one" > testredirect/one.html &&
  echo "my two" > testredirect/two.html &&
  echo "my 404" > testredirect/404.html &&
  mkdir testredirect/redirected-splat &&
  echo "redirected splat one" > testredirect/redirected-splat/one.html &&
  echo "/redirect-one /one.html" > testredirect/_redirects &&
  echo "/301-redirect-one /one.html 301" >> testredirect/_redirects &&
  echo "/302-redirect-two /two.html 302" >> testredirect/_redirects &&
  echo "/200-index /index.html 200" >> testredirect/_redirects &&
  echo "/posts/:year/:month/:day/:title /articles/:year/:month/:day/:title 301" >> testredirect/_redirects &&
  echo "/splat/:splat /redirected-splat/:splat 301" >> testredirect/_redirects &&
  echo "/en/* /404.html 404" >> testredirect/_redirects &&
  echo "/* /index.html 200" >> testredirect/_redirects &&
  REDIRECTS_DIR_CID=$(ipfs add -Qr --cid-version 1 testredirect)
'

REDIRECTS_DIR_HOSTNAME="${REDIRECTS_DIR_CID}.ipfs.localhost:$GWAY_PORT"

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/redirect-one redirects with default of 301, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/redirect-one" > response &&
  test_should_contain "one.html" response &&
  test_should_contain "301 Moved Permanently" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/301-redirect-one redirects with 301, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/301-redirect-one" > response &&
  test_should_contain "one.html" response &&
  test_should_contain "301 Moved Permanently" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/302-redirect-two redirects with 302, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/302-redirect-two" > response &&
  test_should_contain "two.html" response &&
  test_should_contain "302 Found" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/200-index returns 200, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/200-index" > response &&
  test_should_contain "my index" response &&
  test_should_contain "200 OK" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/posts/:year/:month/:day/:title redirects with 301 and placeholders, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/posts/2022/01/01/hello-world" > response &&
  test_should_contain "/articles/2022/01/01/hello-world" response &&
  test_should_contain "301 Moved Permanently" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/splat/one.html redirects with 301 and splat placeholder, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/splat/one.html" > response &&
  test_should_contain "/redirected-splat/one.html" response &&
  test_should_contain "301 Moved Permanently" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/en/has-no-redirects-entry returns custom 404, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/en/has-no-redirects-entry" > response &&
  test_should_contain "404 Not Found" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/catch-all returns 200, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/catch-all" > response &&
  test_should_contain "200 OK" response
'

test_expect_success "request for http://127.0.0.1:$GWAY_PORT/ipfs/$REDIRECTS_DIR_CID/301-redirect-one returns 404, no _redirects since no origin isolation" '
  curl -sD - "http://127.0.0.1:$GWAY_PORT/ipfs/$REDIRECTS_DIR_CID/301-redirect-one" > response &&
  test_should_contain "404 Not Found" response
'

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
export IPFS_NS_MAP="$DNSLINK_FQDN:/ipfs/$REDIRECTS_DIR_CID,$ONLY_DNSLINK_FQDN:/ipfs/$REDIRECTS_DIR_CID"

# restart daemon to apply config changes
test_launch_ipfs_daemon

# make sure test setup is valid (fail if CoreAPI is unable to resolve)
test_expect_success "spoofed DNSLink record resolves in cli" "
  ipfs resolve /ipns/$DNSLINK_FQDN > result &&
  test_should_contain \"$REDIRECTS_DIR_CID\" result &&
  ipfs cat /ipns/$DNSLINK_FQDN/_redirects > result &&
  test_should_contain \"index.html\" result
"

test_done
