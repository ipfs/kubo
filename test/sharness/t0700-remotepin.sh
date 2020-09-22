#!/usr/bin/env bash

test_description="Test ipfs remote pinning operations"

. lib/test-lib.sh

test_remote_pins() {
  BASE=$1
  if [ -n "$BASE" ]; then
    BASE_ARGS="--cid-base=$BASE"
  fi

  test_expect_success "create some hashes using base $BASE" '
    HASH_A=$(echo "A" | ipfs add $BASE_ARGS -q --pin=false)
  '

  test_expect_success "make sure we are connected to pinning service" '
    ipfs pin remote ls --enc=json
  '

  test_expect_success "'ipfs pin remote add'" '
    ID_A=$(ipfs pin remote add --enc=json $BASE_ARGS --name=name_a $HASH_A | jq --raw-output .ID)
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by name" '
    FOUND_ID_A=$(ipfs pin remote ls --enc=json --name=name_a --cid=$HASH_A | jq --raw-output .ID | grep $ID_A) &&
    echo $ID_A > expected &&
    echo $FOUND_ID_A > actual &&
    test_cmp expected actual
  '

  test_expect_success "'ipfs pin remote rm' an existing pin by ID" '
    ipfs pin remote rm --enc=json $ID_A
  '

  test_expect_success "'ipfs pin remote ls' for deleted pin" '
    ipfs pin remote ls --enc=json --name=name_a | jq --raw-output .ID > list
    test_expect_code 1 grep $ID_A list
  '
}

test_init_ipfs

if [ -z "$IPFS_REMOTE_PIN_SERVICE" ] && [ -z "$IPFS_REMOTE_PIN_KEY" ]; then
        # create user on pinning service
        echo "Creating test user on remote pinning service"
        IPFS_REMOTE_PIN_SERVICE=localhost:5000
        IPFS_REMOTE_PIN_KEY=$(curl -X POST $IPFS_REMOTE_PIN_SERVICE/users -d email=sharness@ipfs.io | jq --raw-output '.access_token')
else
        echo "Using remote pinning service from environment"
fi

test_remote_pins ""

# test_kill_ipfs_daemon

test_done
