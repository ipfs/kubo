#!/usr/bin/env bash

test_description="Test ipfs remote pinning operations"

. lib/test-lib.sh

if [ -z ${DOCKER_HOST+x} ]; then
  # TODO: set up instead of skipping?
  skip_all='Skipping pinning service integration tests: missing DOCKER_HOST, remote pinning service not available'
  test_done
fi

# daemon running in online mode to ensure Pin.origins/PinStatus.delegates work
test_init_ipfs
test_launch_ipfs_daemon

TEST_PIN_SVC="http://${DOCKER_HOST}:5000/api/v1"
TEST_PIN_SVC_KEY=$(curl -s -X POST "$TEST_PIN_SVC/users" -d email="go-ipfs-sharness@ipfs.example.com" | jq --raw-output .access_token)

# create user on pinning service
test_expect_success "creating test user on remote pinning service" '
  echo CI host IP address ${TEST_PIN_SVC} &&
  ipfs pin remote service add test_pin_svc ${TEST_PIN_SVC} ${TEST_PIN_SVC_KEY} &&
  ipfs pin remote service add test_invalid_key_svc ${TEST_PIN_SVC} fake_api_key &&
  ipfs pin remote service add test_invalid_url_path_svc ${TEST_PIN_SVC}/invalid-path fake_api_key &&
  ipfs pin remote service add test_invalid_url_dns_svc https://invalid-service.example.com fake_api_key
'

test_expect_success "test 'ipfs pin remote service ls'" '
  ipfs pin remote service ls | tee ls_out &&
  grep -q test_pin_svc ls_out &&
  grep -q test_invalid_key_svc ls_out &&
  grep -q test_invalid_url_path_svc ls_out &&
  grep -q test_invalid_url_dns_svc ls_out
'

test_expect_success "pin remote service ls --pin-count' returns numbers for a valid service" '
  ipfs pin remote service ls --pin-count | grep -E "^test_pin_svc.+[0-9]+/[0-9]+/[0-9]+/[0-9]+$"
'

test_expect_success "pin remote service ls --pin-count' returns 'offline' for invalid service" '
  ipfs pin remote service ls --pin-count | grep -E "^test_invalid_url_path_svc.+offline$"
'

test_expect_success "check connection to test pinning service" '
  ipfs pin remote ls --service=test_pin_svc --enc=json
'

# TODO: improve error returned below
test_expect_success "unathorized pinning service calls fail" '
  test_expect_code 1 ipfs pin remote ls --service=test_invalid_key_svc
'

# TODO: improve error returned below
test_expect_success "misconfigured pinning service calls fail (wrong path)" '
  test_expect_code 1 ipfs pin remote ls --service=test_invalid_url_path_svc
'

# TODO: improve error returned below (panic when offline mode)
test_expect_success "misconfigured pinning service calls fail (dns error)" '
  test_expect_code 1 ipfs pin remote ls --service=test_invalid_url_dns_svc
'

test_expect_success "remove pinning service" '
  ipfs pin remote service rm test_invalid_key_svc &&
  ipfs pin remote service rm test_invalid_url_path_svc &&
  ipfs pin remote service rm test_invalid_url_dns_svc
'

test_expect_success "verify pinning service removal works" '
  ipfs pin remote service ls | grep -L test_invalid_key_svc
'

# we leverage the fact that inlined CID can be pinned instantly on the remote service
# (https://github.com/ipfs-shipyard/rb-pinning-service-api/issues/8)
# below test ensures that assumption is correct (before we proceed to actual tests)
test_expect_success "verify that default add (implicit --background=false) works with data inlined in CID" '
  ipfs pin remote add --service=test_pin_svc --name=inlined_null bafkqaaa &&
  ipfs pin remote ls --service=test_pin_svc --enc=json --name=inlined_null --status=pinned | jq --raw-output .Status | tee ls_out &&
  grep -q "pinned" ls_out
'

test_remote_pins() {
  BASE=$1
  if [ -n "$BASE" ]; then
    BASE_ARGS="--cid-base=$BASE"
  fi

  # note: HAS_MISSING is not inlined nor imported to IPFS on purpose, to reliably test 'queued' state
  test_expect_success "create some hashes using base $BASE" '
    export HASH_A=$(echo "A @ $(date)" | ipfs add $BASE_ARGS -q --inline --inline-limit 100 --pin=false) &&
    export HASH_B=$(echo "B @ $(date)" | ipfs add $BASE_ARGS -q --inline --inline-limit 100 --pin=false) &&
    export HASH_C=$(echo "C @ $(date)" | ipfs add $BASE_ARGS -q --inline --inline-limit 100 --pin=false) &&
    export HASH_D=$(echo "D @ $(date)" | ipfs add $BASE_ARGS -q --inline --inline-limit 100 --pin=false) &&
    export HASH_MISSING=$(echo "MISSING FROM IPFS @ $(date)" | ipfs add $BASE_ARGS -q --only-hash)
  '

  test_expect_success "'ipfs pin remote add --background=true'" '
    export ID_A=$(ipfs pin remote add --background=true --service=test_pin_svc --enc=json $BASE_ARGS --name=name_a $HASH_A | jq --raw-output .RequestID) &&
    export ID_B=$(ipfs pin remote add --background=true --service=test_pin_svc --enc=json $BASE_ARGS --name=name_b $HASH_B | jq --raw-output .RequestID)
  '

  test_expect_success "verify background add worked" '
    ipfs pin remote ls --service=test_pin_svc --enc=json $ID_A $ID_B | tee ls_out &&
    grep -q $HASH_A ls_out &&
    grep -q $HASH_B ls_out
  '

  test_expect_success "'ipfs pin remote add --background=false'" '
    export ID_C=$(ipfs pin remote add --background=false --service=test_pin_svc --enc=json $BASE_ARGS --name=name_c $HASH_C | jq --raw-output .RequestID) &&
    export ID_D=$(ipfs pin remote add --background=false --service=test_pin_svc --enc=json $BASE_ARGS --name=name_d $HASH_D | jq --raw-output .RequestID)
  '

  test_expect_success "verify foreground add worked" '
    ipfs pin remote ls --service=test_pin_svc --enc=json $ID_C $ID_D | tee ls_out &&
    grep -q $HASH_C ls_out &&
    grep -q $HASH_D ls_out
  '


  test_expect_success "'ipfs pin remote add' with CID that is not available" '
    export ID_M=$(ipfs pin remote add --background=true --service=test_pin_svc --enc=json $BASE_ARGS --name=name_m $HASH_MISSING | jq --raw-output .RequestID)
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by multiple statuses" '
    ipfs pin remote ls --service=test_pin_svc --enc=json --status=queued,pinning,pinned,failed | tee ls_out &&
    grep -q $HASH_A ls_out &&
    grep -q $HASH_B ls_out &&
    grep -q $HASH_MISSING ls_out
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by RequestID" '
    ipfs pin remote ls --service=test_pin_svc --enc=json $ID_A | tee ls_out &&
    grep -q $HASH_A ls_out
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by CID" '
    ipfs pin remote ls --service=test_pin_svc --enc=json  --cid=$HASH_B | tee ls_out &&
    grep -q $HASH_B ls_out
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by name" '
    ipfs pin remote ls --service=test_pin_svc --enc=json --name=name_a | tee ls_out &&
    grep -q $HASH_A ls_out
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by status" '
    ipfs pin remote ls --service=test_pin_svc --enc=json --status=queued,pinning | tee ls_out &&
    grep -q $HASH_MISSING ls_out
  '

  test_expect_success "'ipfs pin remote rm' an existing pin by RequestID" '
    ipfs pin remote rm --service=test_pin_svc $ID_A
  '

  # --force is required only when more than a single match is found,
  # so we add second pin with the same name to simulate that scenario
  test_expect_success "'ipfs pin remote rm --name' fails without --force when matching multiple pins" '
    export ID_B2=$(ipfs pin remote add --service=test_pin_svc --enc=json $BASE_ARGS --name=name_b $HASH_B | jq --raw-output .RequestID) &&
    test_expect_code 1 ipfs pin remote rm --service=test_pin_svc --enc=json --name=name_b
  '

  test_expect_success "'ipfs pin remote rm --name' without --force did not remove matching pins" '
    ipfs pin remote ls --service=test_pin_svc --enc=json --name=name_b | jq --raw-output .RequestID | tee ls_out &&
    grep -q $ID_B &&
    grep -q $ID_B2
  '

  test_expect_success "'ipfs pin remote rm --force' removes all pinned items" '
    ipfs pin remote rm --service=test_pin_svc --enc=json --force --status=pinned
  '

  test_expect_success "'ipfs pin remote ls' returns error for deleted pin" '
    ipfs pin remote ls --service=test_pin_svc --enc=json $ID_B | tee ls_out &&
    test_expect_code 1 grep -q $HASH_B ls_out
  '

}

test_remote_pins ""

test_kill_ipfs_daemon
test_done
