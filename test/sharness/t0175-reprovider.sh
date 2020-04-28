#!/usr/bin/env bash

test_description="Test reprovider"

. lib/test-lib.sh

NUM_NODES=6

init_strategy() {
  test_expect_success 'init iptb' '
    iptb testbed create -type localipfs -force -count $NUM_NODES -init
  '

  test_expect_success 'peer ids' '
    PEERID_0=$(iptb attr get 0 id) &&
    PEERID_1=$(iptb attr get 1 id)
  '

  test_expect_success 'use pinning strategy for reprovider' '
    ipfsi 0 config Reprovider.Strategy '$1'
  '

  startup_cluster ${NUM_NODES}
}

reprovide() {
  test_expect_success 'reprovide' '
    # TODO: this hangs, though only after reprovision was done
    ipfsi 0 bitswap reprovide
  '
}

# Test 'all' strategy
init_strategy 'all'

test_expect_success 'add test object' '
  HASH_0=$(echo "foo" | ipfsi 0 add -q --local)
'

findprovs_empty '$HASH_0'
reprovide
findprovs_expect '$HASH_0' '$PEERID_0'

test_expect_success 'Stop iptb' '
  iptb stop
'

# Test 'pinned' strategy
init_strategy 'pinned'

test_expect_success 'prepare test files' '
  echo foo > f1 &&
  echo bar > f2
'

test_expect_success 'add test objects' '
  HASH_FOO=$(ipfsi 0 add -q --offline --pin=false f1) &&
  HASH_BAR=$(ipfsi 0 add -q --offline --pin=false f2) &&
  HASH_BAR_DIR=$(ipfsi 0 add -q --offline -w f2)
'

findprovs_empty '$HASH_FOO'
findprovs_empty '$HASH_BAR'
findprovs_empty '$HASH_BAR_DIR'

reprovide

findprovs_empty '$HASH_FOO'
findprovs_expect '$HASH_BAR' '$PEERID_0'
findprovs_expect '$HASH_BAR_DIR' '$PEERID_0'

test_expect_success 'Stop iptb' '
  iptb stop
'

# Test 'roots' strategy
init_strategy 'roots'

test_expect_success 'prepare test files' '
  echo foo > f1 &&
  echo bar > f2 &&
  echo baz > f3
'

test_expect_success 'add test objects' '
  HASH_FOO=$(ipfsi 0 add -q --offline --pin=false f1) &&
  HASH_BAR=$(ipfsi 0 add -q --offline --pin=false f2) &&
  HASH_BAZ=$(ipfsi 0 add -q --offline f3) &&
  HASH_BAR_DIR=$(ipfsi 0 add -q --offline -w f2 | tail -1)
'

findprovs_empty '$HASH_FOO'
findprovs_empty '$HASH_BAR'
findprovs_empty '$HASH_BAR_DIR'

reprovide

findprovs_empty '$HASH_FOO'
findprovs_empty '$HASH_BAR'
findprovs_expect '$HASH_BAZ' '$PEERID_0'
findprovs_expect '$HASH_BAR_DIR' '$PEERID_0'

test_expect_success 'Stop iptb' '
  iptb stop
'

# Test reprovider working with ticking disabled
test_expect_success 'init iptb' '
  iptb testbed create -type localipfs -force -count $NUM_NODES -init
'

test_expect_success 'peer ids' '
  PEERID_0=$(iptb attr get 0 id) &&
  PEERID_1=$(iptb attr get 1 id)
'

test_expect_success 'Disable reprovider ticking' '
  ipfsi 0 config Reprovider.Interval 0
'

startup_cluster ${NUM_NODES}

test_expect_success 'add test object' '
  HASH_0=$(echo "foo" | ipfsi 0 add -q --offline)
'

findprovs_empty '$HASH_0'
reprovide
findprovs_expect '$HASH_0' '$PEERID_0'

test_expect_success 'resolve object $HASH_0' '
  HASH_WITH_PREFIX=$(ipfsi 1 resolve $HASH_0)
'
findprovs_expect '$HASH_WITH_PREFIX' '$PEERID_0'

test_expect_success 'Stop iptb' '
  iptb stop
'

test_done
