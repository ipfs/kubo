#!/usr/bin/env bash

test_description="Test HTTP Gateway Cache Control Support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon_without_network

# Cache control support is based on logical roots (each path segment == one logical root).
# To maximize the test surface, we want to test:
# - /ipfs/ content path
# - /ipns/ content path
# - at least 3 levels
# - separate tests for a directory listing and a file
# - have implicit index.html for a good measure
# /ipns/root1/root2/root3/ (/ipns/root1/root2/root3/index.html)

# Note: we cover important UnixFS-focused edge case here:
#
# ROOT3_CID - dir listing (dir-index-html response)
# ROOT4_CID - index.html returned as a root response (dir/), instead of generated dir-index-html
# FILE_CID  - index.html returned directly, as a file
#
# Caching of things like raw blocks, CARs, dag-json and dag-cbor
# is tested in their respective suites.

ROOT1_CID=bafybeib3ffl2teiqdncv3mkz4r23b5ctrwkzrrhctdbne6iboayxuxk5ui # ./
ROOT2_CID=bafybeih2w7hjocxjg6g2ku25hvmd53zj7og4txpby3vsusfefw5rrg5sii # ./root2
ROOT3_CID=bafybeiawdvhmjcz65x5egzx4iukxc72hg4woks6v6fvgyupiyt3oczk5ja # ./root2/root3
ROOT4_CID=bafybeifq2rzpqnqrsdupncmkmhs3ckxxjhuvdcbvydkgvch3ms24k5lo7q # ./root2/root3/root4
FILE_CID=bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am # ./root2/root3/root4/index.html
TEST_IPNS_ID=k51qzi5uqu5dlxdsdu5fpuu7h69wu4ohp32iwm9pdt9nq3y5rpn3ln9j12zfhe

# Import test case
# See the static fixtures in ./t0116-gateway-cache/
test_expect_success "Add the test directory" '
  ipfs dag import --pin-roots ../t0116-gateway-cache/fixtures.car
  ipfs routing put --allow-offline /ipns/${TEST_IPNS_ID} ../t0116-gateway-cache/${TEST_IPNS_ID}.ipns-record
'

# Etag
    test_expect_success "GET for /ipfs/ unixfs dir listing succeeds" '
      curl -svX GET "http://127.0.0.1:$GWAY_PORT/ipfs/$ROOT1_CID/root2/root3/" >/dev/null 2>curl_ipfs_dir_listing_output
    '

    test_expect_success "GET for /ipns/ unixfs dir listing succeeds" '
      curl -svX GET "http://127.0.0.1:$GWAY_PORT/ipns/$TEST_IPNS_ID/root2/root3/" >/dev/null 2>curl_ipns_dir_listing_output
    '

    ## dir generated listing
    test_expect_success "GET /ipfs/ dir response has special Etag for generated dir listing" '
    test_should_contain "< Etag: \"DirIndex" curl_ipfs_dir_listing_output &&
    grep -E "< Etag: \"DirIndex-.+_CID-${ROOT3_CID}\"" curl_ipfs_dir_listing_output
    '
    test_expect_success "GET /ipns/ dir response has special Etag for generated dir listing" '
    test_should_contain "< Etag: \"DirIndex" curl_ipns_dir_listing_output &&
    grep -E "< Etag: \"DirIndex-.+_CID-${ROOT3_CID}\"" curl_ipns_dir_listing_output
    '


test_kill_ipfs_daemon

test_done
