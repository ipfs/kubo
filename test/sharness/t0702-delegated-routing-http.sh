#!/usr/bin/env bash

test_description="Test delegated routing via HTTP endpoint"

. lib/test-lib.sh

if ! test_have_prereq SOCAT; then
  skip_all="skipping '$test_description': socat is not available"
  test_done
fi

# simple http routing server mock
# local endpoint responds with deterministic application/vnd.ipfs.rpc+dag-json; version=1
HTTP_ROUTING_PORT=5098
function start_http_routing_mock_endpoint() {
    REMOTE_SERVER_LOG="http-routing-server.log"
    rm -f $REMOTE_SERVER_LOG

    touch response
    socat tcp-listen:$HTTP_ROUTING_PORT,fork,bind=127.0.0.1,reuseaddr 'SYSTEM:cat response'!!CREATE:$REMOTE_SERVER_LOG &
    REMOTE_SERVER_PID=$!

    socat /dev/null tcp:127.0.0.1:$HTTP_ROUTING_PORT,retry=10
    return $?
}
function serve_http_routing_response() {
    local body=$1
    local status_code=${2:-"200 OK"}
    local length=$((1 + ${#body}))
    echo -e "HTTP/1.1 $status_code\nContent-Length: $length\nContent-Type: application/json\n\n$body" > response
}
function stop_http_routing_mock_endpoint() {
    exec 7<&-
    kill $REMOTE_SERVER_PID > /dev/null 2>&1
    wait $REMOTE_SERVER_PID || true
}

# daemon running in online mode to ensure Pin.origins/PinStatus.delegates work
test_init_ipfs

# based on static, synthetic http routing messages:
# t0702-delegated-routing-http/FindProvidersRequest
# t0702-delegated-routing-http/FindProvidersResponse
FINDPROV_CID="baeabep4vu3ceru7nerjjbk37sxb7wmftteve4hcosmyolsbsiubw2vr6pqzj6mw7kv6tbn6nqkkldnklbjgm5tzbi4hkpkled4xlcr7xz4bq"
EXPECTED_PROV="12D3KooWARYacCc6eoCqvsS9RW9MA2vo51CV75deoiqssx3YgyYJ"

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

ipfs config Routing.Type --json '"custom"' || exit 1
ipfs config Routing.Methods --json '{
      "find-peers": {
        "RouterName": "TestDelegatedRouter"
      },
      "find-providers": {
        "RouterName": "TestDelegatedRouter"
      },
      "get-ipns": {
        "RouterName": "TestDelegatedRouter"
      },
      "provide": {
        "RouterName": "TestDelegatedRouter"
      }
    }' || exit 1

test_expect_success "missing method params makes daemon fails" '
  echo "Error: constructing the node (see log for full detail): method name \"put-ipns\" is missing from Routing.Methods config param" > expected_error &&
  GOLOG_LOG_LEVEL=fatal ipfs daemon 2> actual_error || exit 0 &&
  test_cmp expected_error actual_error
'

ipfs config Routing.Methods --json '{
      "find-peers": {
        "RouterName": "TestDelegatedRouter"
      },
      "find-providers": {
        "RouterName": "TestDelegatedRouter"
      },
      "get-ipns": {
        "RouterName": "TestDelegatedRouter"
      },
      "provide": {
        "RouterName": "TestDelegatedRouter"
      },
      "put-ipns": {
        "RouterName": "TestDelegatedRouter"
      },
      "NOT_SUPPORTED": {
        "RouterName": "TestDelegatedRouter"
      }
    }' || exit 1

test_expect_success "having wrong methods makes daemon fails" '
  echo "Error: constructing the node (see log for full detail): method name \"NOT_SUPPORTED\" is not a supported method on Routing.Methods config param" > expected_error &&
  GOLOG_LOG_LEVEL=fatal ipfs daemon 2> actual_error || exit 0 &&
  test_cmp expected_error actual_error
'

# set Routing config to only use delegated routing via mocked http routing endpoint

ipfs config Routing.Type --json '"custom"' || exit 1
ipfs config Routing.Routers.TestDelegatedRouter --json '{
  "Type": "http",
  "Parameters": {
    "Endpoint": "http://127.0.0.1:5098/routing/v1"
  }
}' || exit 1
ipfs config Routing.Methods --json '{
      "find-peers": {
        "RouterName": "TestDelegatedRouter"
      },
      "find-providers": {
        "RouterName": "TestDelegatedRouter"
      },
      "get-ipns": {
        "RouterName": "TestDelegatedRouter"
      },
      "provide": {
        "RouterName": "TestDelegatedRouter"
      },
      "put-ipns": {
        "RouterName": "TestDelegatedRouter"
      }
    }' || exit 1

test_expect_success "adding http delegated routing endpoint to Routing.Routers config works" '
  echo "http://127.0.0.1:5098/routing/v1" > expected &&
  ipfs config Routing.Routers.TestDelegatedRouter.Parameters.Endpoint > actual &&
  test_cmp expected actual
'

test_launch_ipfs_daemon

test_expect_success "start_http_routing_mock_endpoint" '
  start_http_routing_mock_endpoint
'

test_expect_success "'ipfs routing findprovs' returns result from delegated http router" '
  serve_http_routing_response "$(<../t0702-delegated-routing-http/FindProvidersResponse)" &&
  echo "$EXPECTED_PROV" > expected &&
  ipfs routing findprovs "$FINDPROV_CID" > actual &&
  test_cmp expected actual
'

test_expect_success "stop_http_routing_mock_endpoint" '
  stop_http_routing_mock_endpoint
'


test_kill_ipfs_daemon
test_done
# vim: ts=2 sw=2 sts=2 et:
