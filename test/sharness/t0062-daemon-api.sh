#!/bin/sh
#
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test daemon command"

. lib/test-lib.sh

test_init_ipfs

differentport=$((PORT_API + 1))
api_different="/ip4/127.0.0.1/tcp/$differentport"
api_unreachable="/ip4/127.0.0.1/tcp/1"
api_fromcfg=$(ipfs config Addresses.API)
peerid=$(ipfs config Identity.PeerID)

test_client() {
	printf $peerid >expected
	ipfs "$@" id -f="<id>" >actual
	test_cmp expected actual
}

test_client_must_fail() {
	echo "Error: api not running" >expected_err
	test_must_fail ipfs "$@" id -f="<id>" >actual 2>actual_err
	test_cmp expected_err actual_err
}


# first, test things without daemon, without /api file

test_expect_success "client should work (daemon off, no /api file, no --api)" '
	test_client
'

test_expect_success "client --api fromcfg should err (daemon off, no /api file)" '
	test_client_must_fail --api "$api_fromcfg"
'

test_expect_success "client --api unreachable should err (daemon off, no /api file)" '
	test_client_must_fail --api "$api_unreachable"
'

# then, test things with daemon, with /api file

test_launch_ipfs_daemon

test_expect_success "'ipfs daemon' creates api file" '
	test -f ".ipfs/api"
'

test_expect_success "api file looks good" '
	printf "$ADDR_API" >expected &&
	test_cmp expected .ipfs/api
'

test_expect_success "client should work (daemon on, /api file, no --api)" '
	test_client
'

test_expect_success "client --api fromcfg should work (daemon used cfg) (daemon, /api file)" '
	test_client --api "$api_fromcfg"
'

test_expect_success "client --api unreachable should err (daemon, /api file)" '
	test_client_must_fail --api "$api_unreachable"
'

# then, test things without daemon, with /api file

test_kill_ipfs_daemon

test_expect_success "client should work (daemon off, /api file, no --api)" '
	test_client
'

test_expect_success "client --api fromcfg should err (daemon off, /api file)" '
	test_client_must_fail --api "$api_fromcfg"
'

test_expect_success "client --api unreachable should err (daemon, /api file)" '
	test_client_must_fail --api "$api_unreachable"
'

# then, test things with daemon --api $api_different, with /api file

PORT_API=$differentport
ADDR_API=$api_different

test_launch_ipfs_daemon --api "$ADDR_API"

test_expect_success "'ipfs daemon' --api option works" '
	printf "$api_different" >expected &&
	test_cmp expected .ipfs/api
'

test_expect_success "client should work (daemon on, /api file (different), no --api)" '
	test_client
'

test_expect_success "client --api different should work (daemon on, /api file (different))" '
	test_client --api "$api_different"
'

test_expect_success "client --api fromcfg should err (daemon on, /api file (different))" '
	test_client_must_fail --api "$api_fromcfg"
'

test_expect_success "client --api unreachable should err (daemon, /api file)" '
	test_client_must_fail --api "$api_unreachable"
'

# then, test things with daemon off, with /api file, for good measure.

test_kill_ipfs_daemon

test_expect_success "client should work (daemon off, /api file (different), no --api)" '
	test_client
'

test_expect_success "client --api different should work (daemon on, /api file (different))" '
	test_client_must_fail --api "$api_different"
'

test_expect_success "client --api fromcfg should err (daemon on, /api file (different))" '
	test_client_must_fail --api "$api_fromcfg"
'

test_expect_success "client --api unreachable should err (daemon, /api file)" '
	test_client_must_fail --api "$api_unreachable"
'

test_done
