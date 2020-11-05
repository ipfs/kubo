#!/usr/bin/env bash

test_description="Test ipfs remote pinning operations"

. lib/test-lib.sh

test_init_ipfs

# create user on pinning service
test_expect_success "creating test user on remote pinning service" '
        echo CI host IP address ${CI_HOST_IP} &&
        export TEST_PIN_SVC=http://${CI_HOST_IP}:5000/api/v1 &&
        ipfs pin remote service add test_pin_svc $TEST_PIN_SVC $(curl -X POST $TEST_PIN_SVC/users -d email=sharness@ipfs.io | jq --raw-output .access_token) &&
        ipfs pin remote service add fake_pin_svc http://0.0.0.0:5000 fake_api_key
'

test_expect_success "test 'ipfs pin remote service ls'"'
  ipfs pin remote service ls | jq --raw-output .Service | grep test_pin_svc &&
  ipfs pin remote service ls | jq --raw-output .Service | grep fake_pin_svc
'

test_expect_success "check connection to test pinning service" '
  ipfs pin remote ls --service=test_pin_svc --enc=json
'

test_expect_failure "test fake pinning service calls fail" '
  ipfs pin remote ls --service=fake_pin_svc --enc=json
'

test_expect_success "remove fake pinning service" '
  ipfs pin remote service rm fake_pin_svc
'

test_expect_failure "verify fake pinning service removed"'
  ipfs pin remote service ls | jq --raw-output .Service | grep fake_pin_svc
'

test_remote_pins() {
  BASE=$1
  if [ -n "$BASE" ]; then
    BASE_ARGS="--cid-base=$BASE"
  fi

  test_expect_success "create some hashes using base $BASE" '
    export HASH_A=$(echo "A" | ipfs add $BASE_ARGS -q --pin=false) &&
    export HASH_B=$(echo "B" | ipfs add $BASE_ARGS -q --pin=false) &&
    export HASH_C=$(echo "C" | ipfs add $BASE_ARGS -q --pin=false) &&
    export HASH_D=$(echo "D" | ipfs add $BASE_ARGS -q --pin=false)
  '

  test_expect_success "verify background add works" '
    export ID_D=$(ipfs pin remote add --background=true --service=test_pin_svc --enc=json $BASE_ARGS --name=name_d $HASH_D | jq --raw-output .RequestID) &&
    sleep 3 &&
    ipfs pin remote ls --service=test_pin_svc --enc=json --name=name_d | jq --raw-output .Status | grep pinned
  '

  test_expect_success "'ipfs pin remote add'" '
    export ID_A=$(ipfs pin remote add --background=false --service=test_pin_svc --enc=json $BASE_ARGS --name=name_a $HASH_A | jq --raw-output .RequestID) &&
    export ID_B=$(ipfs pin remote add --background=false --service=test_pin_svc --enc=json $BASE_ARGS --name=name_b $HASH_B | jq --raw-output .RequestID) &&
    export ID_C=$(ipfs pin remote add --background=false --service=test_pin_svc --enc=json $BASE_ARGS --name=name_c $HASH_C | jq --raw-output .RequestID)
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by ID" '
    FOUND_ID_A=$(ipfs pin remote ls --service=test_pin_svc --enc=json --cid=$HASH_A | jq --raw-output .RequestID | grep $ID_A) &&
    echo ID_A=$ID_A FOUND_ID_A=$FOUND_ID_A &&
    echo $ID_A > expected &&
    echo $FOUND_ID_A > actual &&
    test_cmp expected actual
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by name" '
    FOUND_ID_A=$(ipfs pin remote ls --service=test_pin_svc --enc=json --name=name_a | jq --raw-output .RequestID | grep $ID_A) &&
    echo ID_A=$ID_A FOUND_ID_A=$FOUND_ID_A &&
    echo $ID_A > expected &&
    echo $FOUND_ID_A > actual &&
    test_cmp expected actual
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by status" '
    FOUND_ID_A=$(ipfs pin remote ls --service=test_pin_svc --enc=json --status=pinned | jq --raw-output .RequestID | grep $ID_A) &&
    echo ID_A=$ID_A FOUND_ID_A=$FOUND_ID_A &&
    echo $ID_A > expected &&
    echo $FOUND_ID_A > actual &&
    test_cmp expected actual
  '

  test_expect_success "'ipfs pin remote rm' an existing pin by ID" '
    ipfs pin remote rm --service=test_pin_svc --enc=json $ID_A
  '

  test_expect_success "'ipfs pin remote rm' an existing pin by name" '
    ipfs pin remote rm --service=test_pin_svc --enc=json --name=name_b
  '

  test_expect_success "'ipfs pin remote rm' an existing pin by status" '
    ipfs pin remote rm --service=test_pin_svc --enc=json --status=pinned
  '

  test_expect_success "'ipfs pin remote ls' for deleted pin" '
    ipfs pin remote ls --service=test_pin_svc --enc=json --name=name_a | jq --raw-output .RequestID > list
    test_expect_code 1 grep $ID_A list
  '
}

test_remote_pins ""

# test_launch_ipfs_daemon
# test_kill_ipfs_daemon

test_done
