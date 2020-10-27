#!/usr/bin/env bash

test_description="Test ipfs remote pinning operations"

. lib/test-lib.sh

test_remote_pins() {
  BASE=$1
  if [ -n "$BASE" ]; then
    BASE_ARGS="--cid-base=$BASE"
  fi

  echo Using IPFS_REMOTE_PIN_SERVICE=$IPFS_REMOTE_PIN_SERVICE
  echo Using IPFS_REMOTE_PIN_KEY=$IPFS_REMOTE_PIN_KEY

  test_expect_success "create some hashes using base $BASE" '
    export HASH_A=$(echo "A" | ipfs add $BASE_ARGS -q --pin=false)
  '

  test_expect_success "check connection to pinning service" '
    ipfs pin remote ls --enc=json
  '

  test_expect_success "'ipfs pin remote add'" '
    export ID_A=$(ipfs pin remote add --enc=json $BASE_ARGS --name=name_a $HASH_A | jq --raw-output .RequestID) &&
    sleep 3 # provide time for the pinning service to download the file
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by name" '
    FOUND_ID_A=$(ipfs pin remote ls --enc=json --name=name_a --cid=$HASH_A | jq --raw-output .RequestID | grep $ID_A) &&
    echo ID_A=$ID_A FOUND_ID_A=$FOUND_ID_A &&
    echo $ID_A > expected &&
    echo $FOUND_ID_A > actual &&
    test_cmp expected actual
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by status" '
    FOUND_ID_A=$(ipfs pin remote ls --enc=json --status=pinned | jq --raw-output .RequestID | grep $ID_A) &&
    echo ID_A=$ID_A FOUND_ID_A=$FOUND_ID_A &&
    echo $ID_A > expected &&
    echo $FOUND_ID_A > actual &&
    test_cmp expected actual
  '

  test_expect_success "'ipfs pin remote rm' an existing pin by ID" '
    ipfs pin remote rm --enc=json $ID_A
  '

  test_expect_success "'ipfs pin remote ls' for deleted pin" '
    ipfs pin remote ls --enc=json --name=name_a | jq --raw-output .RequestID > list
    test_expect_code 1 grep $ID_A list
  '
}

test_init_ipfs

# create user on pinning service
test_expect_success "creating test user on remote pinning service" '
        echo CI host IP address ${CI_HOST_IP} &&
        export IPFS_REMOTE_PIN_SERVICE=http://${CI_HOST_IP}:5000/api/v1 &&
        export IPFS_REMOTE_PIN_KEY=$(curl -X POST $IPFS_REMOTE_PIN_SERVICE/users -d email=sharness@ipfs.io | jq --raw-output .access_token)
'

test_remote_pins ""

# test_kill_ipfs_daemon

test_done
