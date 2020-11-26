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

# create user on pinning service
TEST_PIN_SVC="http://${DOCKER_HOST}:5000/api/v1"
TEST_PIN_SVC_KEY=$(curl -s -X POST "$TEST_PIN_SVC/users" -d email="go-ipfs-sharness@ipfs.example.com" | jq --raw-output .access_token)

# pin remote service  add|ls|rm

# add valid and invalid services
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

# SECURITY of access tokens in ApiKey fields:
# Pinning.RemoteServices includes ApiKey, and we give it the same treatment
# as Identity.PrivKey to prevent exposing it on the network

test_expect_success "'ipfs config Pinning' fails" '
  test_expect_code 1 ipfs config Pinning 2> config_out
'
test_expect_success "output includes meaningful error" '
  echo "Error: cannot show or change pinning services through this API (try: ipfs pin remote service --help)" > config_exp &&
  test_cmp config_exp config_out
'

test_expect_success "'ipfs config Pinning.RemoteServices' fails" '
  test_expect_code 1 ipfs config Pinning.RemoteServices 2> config_out
'
test_expect_success "output includes meaningful error" '
  test_cmp config_exp config_out
'

test_expect_success "'ipfs config Pinning.RemoteServices.test_pin_svc' fails" '
  test_expect_code 1 ipfs config Pinning.RemoteServices.test_pin_svc 2> config_out
'
test_expect_success "output includes meaningful error" '
  test_cmp config_exp config_out
'

test_expect_success "'ipfs config show' doesn't include RemoteServices" '
  ipfs config show > show_config &&
  test_expect_code 1 grep RemoteServices show_config
'

test_expect_success "'ipfs config replace' injects remote services back" '
  test_expect_code 1 grep -q -E "test_.+_svc" show_config &&
  ipfs config replace show_config &&
  test_expect_code 0 grep -q test_pin_svc "$IPFS_PATH/config" &&
  test_expect_code 0 grep -q test_invalid_key_svc "$IPFS_PATH/config" &&
  test_expect_code 0 grep -q test_invalid_url_path_svc "$IPFS_PATH/config" &&
  test_expect_code 0 grep -q test_invalid_url_dns_svc "$IPFS_PATH/config"
'

# note: we remove Identity.PrivKey to ensure error is triggered by Pinning.RemoteServices
test_expect_success "'ipfs config replace' with remote services errors out" '
  jq -M "del(.Identity.PrivKey)" "$IPFS_PATH/config" | jq ".Pinning += { RemoteServices: {\"foo\": {} }}" > new_config &&
  test_expect_code 1 ipfs config replace - < new_config 2> replace_out
'
test_expect_success "output includes meaningful error" '
  echo "Error: cannot show or change pinning services through this API (try: ipfs pin remote service --help)" > replace_expected
  test_cmp replace_out replace_expected
'

# /SECURITY

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

# pin remote service rm

test_expect_success "remove pinning service" '
  ipfs pin remote service rm test_invalid_key_svc &&
  ipfs pin remote service rm test_invalid_url_path_svc &&
  ipfs pin remote service rm test_invalid_url_dns_svc
'

test_expect_success "verify pinning service removal works" '
  ipfs pin remote service ls | grep -L test_invalid_key_svc
'

# pin remote add

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
    export HASH_MISSING=$(echo "MISSING FROM IPFS @ $(date)" | ipfs add $BASE_ARGS -q --only-hash) &&
    echo "A: $HASH_A" &&
    echo "B: $HASH_B" &&
    echo "M: $HASH_MISSING"
  '

  test_expect_success "'ipfs pin remote add --background=true'" '
    export ID_A=$(ipfs pin remote add --background=true --service=test_pin_svc --enc=json $BASE_ARGS --name=name_a $HASH_A | jq --raw-output .RequestID)
  '

  test_expect_success "'ipfs pin remote add --background=true' with CID that is not available" '
    export ID_M=$(ipfs pin remote add --background=true --service=test_pin_svc --enc=json $BASE_ARGS --name=name_m $HASH_MISSING | jq --raw-output .RequestID)
  '

  test_expect_success "'ipfs pin remote ls' includes queued CID that is not available" '
    ipfs pin remote ls --service=test_pin_svc --enc=json $ID_M
  '

  test_expect_success "verify background add worked" '
    ipfs pin remote ls --service=test_pin_svc --enc=json $ID_A $ID_M | tee ls_out &&
    test_expect_code 0 grep -q $HASH_A ls_out &&
    test_expect_code 0 grep -q $HASH_MISSING ls_out
  '

  test_expect_success "'ipfs pin remote add --background=false'" '
    export ID_B=$(ipfs pin remote add --background=false --service=test_pin_svc --enc=json $BASE_ARGS --name=name_b $HASH_B | jq --raw-output .RequestID)
  '

  test_expect_success "verify foreground add worked" '
    ipfs pin remote ls --service=test_pin_svc --enc=json $ID_B | tee ls_out &&
    test_expect_code 0 grep -q $HASH_B ls_out
  '

  # TODO: this does not seem to find $HASH_MISSING
  test_expect_success "'ipfs pin remote ls' for existing pins by multiple statuses" '
    ipfs pin remote ls --service=test_pin_svc --enc=json --status=queued,pinning,pinned,failed | tee ls_out &&
    test_expect_code 0 grep -q $HASH_A ls_out &&
    test_expect_code 0 grep -q $HASH_B ls_out &&
    test_expect_code 0 grep -q $HASH_MISSING ls_out
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by RequestID" '
    ipfs pin remote ls --service=test_pin_svc --enc=json $ID_A | tee ls_out &&
    test_expect_code 0 grep -q $HASH_A ls_out
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by CID" '
    ipfs pin remote ls --service=test_pin_svc --enc=json  --cid=$HASH_B | tee ls_out &&
    test_expect_code 0 grep -q $HASH_B ls_out
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by name" '
    ipfs pin remote ls --service=test_pin_svc --enc=json --name=name_a | tee ls_out &&
    test_expect_code 0 grep -q $HASH_A ls_out
  '

  # TODO: this does not seem to find $HASH_MISSING
  test_expect_success "'ipfs pin remote ls' for ongoing pins by status" '
    ipfs pin remote ls --service=test_pin_svc --status=queued,pinning | tee ls_out &&
    test_expect_code 0 grep -q $HASH_MISSING ls_out
  '

  test_expect_success "'ipfs pin remote rm' an existing pin by RequestID" '
    test_expect_code 0 ipfs pin remote ls --service=test_pin_svc $ID_A &&
    ipfs pin remote rm --service=test_pin_svc $ID_A &&
    test_expect_code 1 ipfs pin remote ls --service=test_pin_svc $ID_A
  '

  # --force is required only when more than a single match is found,
  # so we add second pin with the same name to simulate that scenario
  test_expect_success "'ipfs pin remote rm --name' fails without --force when matching multiple pins" '
    export ID_B2=$(ipfs pin remote add --service=test_pin_svc --enc=json $BASE_ARGS --name=name_b $HASH_B | jq --raw-output .RequestID) &&
    test_expect_code 1 ipfs pin remote rm --service=test_pin_svc --enc=json --name=name_b
  '

  test_expect_success "'ipfs pin remote rm --name' without --force did not remove matching pins" '
    ipfs pin remote ls --service=test_pin_svc --enc=json --name=name_b | jq --raw-output .RequestID | tee ls_out &&
    test_expect_code 0 grep -q $ID_B ls_out &&
    test_expect_code 0 grep -q $ID_B2 ls_out
  '

  test_expect_success "'ipfs pin remote rm --force' removes all pinned items" '
    ipfs pin remote rm --service=test_pin_svc --enc=json --force &&
    ipfs pin remote ls --service=test_pin_svc --enc=json --status=pinned | tee ls_out &&
    test_expect_code 1 grep -q $ID_A ls_out &&
    test_expect_code 1 grep -q $ID_B ls_out
  '

  test_expect_success "'ipfs pin remote ls' returns error for deleted pin" '
    ipfs pin remote ls --service=test_pin_svc --enc=json $ID_B | tee ls_out &&
    test_expect_code 1 grep -q $HASH_B ls_out
  '

}

test_remote_pins ""

test_kill_ipfs_daemon
test_done

# vim: ts=2 sw=2 sts=2 et:
