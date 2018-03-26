#!/bin/sh

test_description="Test provider"

. lib/test-lib.sh

NUM_NODES=6

test_expect_success 'init iptb' '
  iptb init -f -n $NUM_NODES --bootstrap=none --port=0
'

test_expect_success 'peer ids' '
  PEERID_0=$(iptb get id 0) &&
  PEERID_1=$(iptb get id 1)
'

startup_cluster ${NUM_NODES}

findprovs_empty() {
  test_expect_success 'findprovs '$1' succeeds' '
    ipfsi 1 dht findprovs -n 1 '$1' > findprovsOut
  '

  test_expect_success "findprovs $1 output is empty" '
    test_must_be_empty findprovsOut
  '
}

findprovs_expect() {
  test_expect_success 'findprovs '$1' succeeds' '
    ipfsi 1 dht findprovs -n 1 '$1' > findprovsOut &&
    echo '$2' > expected
  '

  test_expect_success "findprovs $1 output looks good" '
    test_cmp findprovsOut expected
  '
}

test_expect_success 'prepare files for ipfs add' '
  random-files -depth=2 -dirs=2 -files=4 -seed=1 d1 > /dev/null &&
  random-files -depth=2 -dirs=2 -files=4 -seed=2 d2 > /dev/null
'

test_expect_success 'ipfs add files' '
  HASH_F1=$(echo 1 | ipfsi 0 add -q --local)
  HASH_F2=$(echo 2 | ipfsi 0 add -q)
'

findprovs_empty '$HASH_F1'
findprovs_expect '$HASH_F2' '$PEERID_0'

test_expect_success 'ipfs add directories' '
  HASH_D1=$(ipfsi 0 add -qr --local d1)
  HASH_D2=$(ipfsi 0 add -qr d2)
'

findprovs_empty '$HASH_D1'
findprovs_expect '$HASH_D2' '$PEERID_0'

test_expect_success 'ipfs block put' '
  HASH_B1=$(echo 1 | ipfsi 0 block put)
'

findprovs_expect '$HASH_B1' '$PEERID_0'

test_expect_success 'ipfs dag put' '
  HASH_C1=$(echo 1 | ipfsi 0 dag put)
'

findprovs_expect '$HASH_C1' '$PEERID_0'
findprovs_empty 'QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n'

test_expect_success 'ipfs object' '
  HASH_O1=$(ipfsi 0 object new) &&
  HASH_O2=$(echo "{\"data\":\"foo\"}" | ipfsi 0 object put -q)
'

findprovs_expect '$HASH_O1' '$PEERID_0'
findprovs_expect '$HASH_O2' '$PEERID_0'

test_done
