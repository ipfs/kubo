#!/bin/sh

test_description="Test dht command"

. lib/test-lib.sh

TEST_DHT_VALUE="CAASpgIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQC8hSwYY1FXqjT5M36O/Q5fBeDhXE5ePvGAeN3MIibfChqQpgqBbXQi1gAp4TQypSTKl/AMy7hfzsKauieim7jHMgIYAB4pLoBQD1qGVn/n7CqzAR3gDg9umIGuAy15oT0uaqMDqSepfnyxEyPDqfDklgvmS/MAwfBHjH2IPcMIaFgZ6d6gVlhmwuH8WVQ/geumDqyKuU9Jy+SUozmxEu2Baylg4fuqxaxoqOiPFZeWKSCFAngFj3NPmLApE0Fy48/eEZ+t7iP6s/raupP4+Jk/AFNDJNos4VxUnLJpZ1g6W5vYkkt1kXbMTaqxFVryCdCW2UEOwEzjGPGkcIE4RJrHAgMBAAE="
TEST_DHT_PATH="/pk/QmepgFW7BHEtU4pZJdxaNiv75mKLLRQnPi1KaaXmQN4V1a"

# start iptb + wait for peering
NUM_NODES=5
test_expect_success 'init iptb' '
  iptb init -n $NUM_NODES --bootstrap=none --port=0
'

startup_cluster $NUM_NODES

test_expect_success 'peer ids' '
  PEERID_0=$(iptb get id 0) &&
  PEERID_2=$(iptb get id 2)
'

# ipfs dht findpeer <peerID>
test_expect_success 'findpeer' '
  ipfsi 1 dht findpeer $PEERID_0 | sort >actual &&
  ipfsi 0 id -f "<addrs>" | cut -d / -f 1-5 | sort >expected &&
  test_cmp actual expected
'

# ipfs dht put <key> <value>
test_expect_success 'put with good keys' '
  echo "$TEST_DHT_VALUE" | b64decode | ipfsi 0 dht put "$TEST_DHT_PATH" | sort >putted &&
  [ -s putted ] ||
  test_fsh cat putted
'

# ipfs dht get <key>
test_expect_success 'get with good keys' '
  HASH="$(echo "hello world" | ipfsi 2 add -q)" &&
  ipfsi 2 name publish "/ipfs/$HASH" &&
  ipfsi 1 dht get "/ipns/$PEERID_2" | grep -aq "/ipfs/$HASH"
'

test_expect_failure 'put with bad keys (issue #4611)' '
  ! ipfsi 0 dht put "foo" "bar" &&
  ! ipfsi 0 dht put "/pk/foo" "bar" &&
  ! ipfsi 0 dht put "/ipns/foo" "bar"
'

test_expect_failure 'get with bad keys (issue #4611)' '
  ! ipfsi 0 dht get "foo" &&
  ! ipfsi 0 dht get "/pk/foo"
'

test_expect_success "add a ref so we can find providers for it" '
  echo "some stuff" > afile &&
  HASH=$(ipfsi 3 add -q afile)
'

# ipfs dht findprovs <key>
test_expect_success 'findprovs' '
  ipfsi 4 dht findprovs $HASH > provs &&
  iptb get id 3 > expected &&
  test_cmp provs expected
'


# ipfs dht query <peerID>
## We query 3 different keys, to statisically lower the chance that the queryer
## turns out to be the closest to what a key hashes to.
# TODO: flaky. tracked by https://github.com/ipfs/go-ipfs/issues/2620
test_expect_success 'query' '
  ipfsi 3 dht query banana >actual &&
  ipfsi 3 dht query apple >>actual &&
  ipfsi 3 dht query pear >>actual &&
  PEERS=$(wc -l actual | cut -d '"'"' '"'"' -f 1) &&
  [ -s actual ] ||
  test_might_fail test_fsh cat actual
'

test_expect_success 'stop iptb' '
  iptb stop
'

test_done
