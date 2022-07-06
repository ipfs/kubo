#!/usr/bin/env bash

test_description="Test delegated routing via reframe endpoint"

. lib/test-lib.sh

if ! test_have_prereq SOCAT; then
  skip_all="skipping '$test_description': socat is not available"
  test_done
fi

# simple reframe server mock
# local endpoint responds with deterministic application/vnd.ipfs.rpc+dag-json; version=1
REFRAME_PORT=5098
function start_reframe_mock_endpoint() {
    REMOTE_SERVER_LOG="reframe-server.log"
    rm -f $REMOTE_SERVER_LOG

    touch response
    socat tcp-listen:$REFRAME_PORT,fork,bind=127.0.0.1,reuseaddr 'SYSTEM:cat response'!!CREATE:$REMOTE_SERVER_LOG &
    REMOTE_SERVER_PID=$!

    socat /dev/null tcp:127.0.0.1:$REFRAME_PORT,retry=10
    return $?
}
function serve_reframe_response() {
    local body=$1
    local status_code=${2:-"200 OK"}
    local length=$((1 + ${#body}))
    echo -e "HTTP/1.1 $status_code\nContent-Type: application/vnd.ipfs.rpc+dag-json; version=1\nContent-Length: $length\n\n$body" > response
}
function stop_reframe_mock_endpoint() {
    exec 7<&-
    kill $REMOTE_SERVER_PID > /dev/null 2>&1
    wait $REMOTE_SERVER_PID || true
}

# daemon running in online mode to ensure Pin.origins/PinStatus.delegates work
test_init_ipfs

# based on static, synthetic reframe messages:
# t0701-delegated-routing-reframe/FindProvidersRequest
# t0701-delegated-routing-reframe/FindProvidersResponse
FINDPROV_CID="bafybeigvgzoolc3drupxhlevdp2ugqcrbcsqfmcek2zxiw5wctk3xjpjwy"
EXPECTED_PROV="QmQzqxhK82kAmKvARFZSkUVS6fo9sySaiogAnx5EnZ6ZmC"

test_expect_success "default Routing config has no Routers defined" '
  echo null > expected &&
  ipfs config show | jq .Routing.Routers > actual &&
  test_cmp expected actual
'

# turn off all implicit routers
ipfs config Routing.Type none || exit 1
test_launch_ipfs_daemon
test_expect_success "disabling default router (dht) works" '
  ipfs config Routing.Type > actual &&
  echo none > expected &&
  test_cmp expected actual
'
test_expect_success "no routers means findprovs returns no results" '
  ipfs routing findprovs "$FINDPROV_CID" > actual &&
  echo -n > expected &&
  test_cmp expected actual
'

test_kill_ipfs_daemon

# set Routing config to only use delegated routing via mocked reframe endpoint
ipfs config Routing.Routers.TestDelegatedRouter --json '{
  "Type": "reframe",
  "Parameters": {
    "Endpoint": "http://127.0.0.1:5098/reframe"
  }
}' || exit 1

test_expect_success "adding reframe endpoint to Routing.Routers config works" '
  echo "http://127.0.0.1:5098/reframe" > expected &&
  ipfs config Routing.Routers.TestDelegatedRouter.Parameters.Endpoint > actual &&
  test_cmp expected actual
'

test_launch_ipfs_daemon

test_expect_success "start_reframe_mock_endpoint" '
  start_reframe_mock_endpoint
'

test_expect_success "'ipfs routing findprovs' returns result from delegated reframe router" '
  serve_reframe_response "$(<../t0701-delegated-routing-reframe/FindProvidersResponse)" &&
  echo "$EXPECTED_PROV" > expected &&
  ipfs routing findprovs "$FINDPROV_CID" > actual &&
  test_cmp expected actual
'

test_expect_success "stop_reframe_mock_endpoint" '
  stop_reframe_mock_endpoint
'


test_kill_ipfs_daemon
test_done
# vim: ts=2 sw=2 sts=2 et:
