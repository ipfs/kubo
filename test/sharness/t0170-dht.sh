#!/usr/bin/env bash

test_description="Test dht command"

. lib/test-lib.sh

TEST_DHT_VALUE="foobar"
TEST_DHT_PATH="/pk/QmbWTwYGcmdyK9CYfNBcfs9nhZs17a6FQ4Y8oea278xx41"

test_dht() {
  NUM_NODES=5

  test_expect_success 'init iptb' '
    rm -rf .iptb/ &&
    iptb init -n $NUM_NODES --bootstrap=none --port=0
  '

  startup_cluster $NUM_NODES "$@"

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
    ipfsi 0 dht put "$TEST_DHT_PATH" "$TEST_DHT_VALUE" | sort >putted &&
    [ -s putted ] ||
    test_fsh cat putted
  '
  
  # ipfs dht get <key>
  test_expect_success 'get with good keys' '
    HASH="$(echo "hello world" | ipfsi 2 add -q)" &&
    ipfsi 2 name publish "/ipfs/$HASH" &&
    ipfsi 1 dht get "/ipns/$PEERID_2" | grep -aq "/ipfs/$HASH"
  '
  
  test_expect_success 'put with bad keys fails (issue #5113)' '
    ipfsi 0 dht put "foo" "bar" >putted
    ipfsi 0 dht put "/pk/foo" "bar" >>putted
    ipfsi 0 dht put "/ipns/foo" "bar" >>putted
    [ ! -s putted ] ||
    test_fsh cat putted
  '
  
  test_expect_failure 'put with bad keys returns error (issue #4611)' '
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
    ipfsi 3 dht query "$(echo banana | ipfsi 3 add -q)" >actual &&
    ipfsi 3 dht query "$(echo apple | ipfsi 3 add -q)" >>actual &&
    ipfsi 3 dht query "$(echo pear | ipfsi 3 add -q)"  >>actual &&
    PEERS=$(wc -l actual | cut -d '"'"' '"'"' -f 1) &&
    [ -s actual ] ||
    test_might_fail test_fsh cat actual
  '

  test_expect_success 'stop iptb' '
    iptb stop
  '
}

test_dht
test_dht --enable-pubsub-experiment --enable-namesys-pubsub

test_done
