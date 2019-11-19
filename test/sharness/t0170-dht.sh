#!/usr/bin/env bash

test_description="Test dht command"

. lib/test-lib.sh

test_dht() {
  NUM_NODES=5

  test_expect_success 'init iptb' '
    rm -rf .iptb/ &&
    iptb testbed create -type localipfs -count $NUM_NODES -init
  '

  startup_cluster $NUM_NODES $@

  test_expect_success 'peer ids' '
    PEERID_0=$(iptb attr get 0 id) &&
    PEERID_2=$(iptb attr get 2 id)
  '
  
  # ipfs dht findpeer <peerID>
  test_expect_success 'findpeer' '
    ipfsi 1 dht findpeer $PEERID_0 | sort >actual &&
    ipfsi 0 id -f "<addrs>" | cut -d / -f 1-5 | sort >expected &&
    test_cmp actual expected
  '
  
  # ipfs dht get <key>
  test_expect_success 'get with good keys works' '
    HASH="$(echo "hello world" | ipfsi 2 add -q)" &&
    ipfsi 2 name publish "/ipfs/$HASH" &&
    ipfsi 1 dht get "/ipns/$PEERID_2" >get_result
  '

  test_expect_success 'get with good keys contains the right value' '
    cat get_result | grep -aq "/ipfs/$HASH"
  '

  test_expect_success 'put round trips (#3124)' '
    ipfsi 0 dht put "/ipns/$PEERID_2" get_result | sort >putted &&
    [ -s putted ] ||
    test_fsh cat putted
  '
  
  test_expect_success 'put with bad keys fails (issue #5113)' '
    ipfsi 0 dht put "foo" <<<bar >putted
    ipfsi 0 dht put "/pk/foo" <<<bar >>putted
    ipfsi 0 dht put "/ipns/foo" <<<bar >>putted
    [ ! -s putted ] ||
    test_fsh cat putted
  '
  
  test_expect_success 'put with bad keys returns error (issue #4611)' '
    test_must_fail ipfsi 0 dht put "foo" <<<bar &&
    test_must_fail ipfsi 0 dht put "/pk/foo" <<<bar &&
    test_must_fail ipfsi 0 dht put "/ipns/foo" <<<bar
  '
  
  test_expect_success 'get with bad keys (issue #4611)' '
    test_must_fail ipfsi 0 dht get "foo" &&
    test_must_fail ipfsi 0 dht get "/pk/foo"
  '
  
  test_expect_success "add a ref so we can find providers for it" '
    echo "some stuff" > afile &&
    HASH=$(ipfsi 3 add -q afile)
  '
  
  # ipfs dht findprovs <key>
  test_expect_success 'findprovs' '
    ipfsi 4 dht findprovs $HASH > provs &&
    iptb attr get 3 id > expected &&
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

  test_expect_success "dht commands fail when offline" '
    test_must_fail ipfsi 0 dht findprovs "$HASH" 2>err_findprovs &&
    test_must_fail ipfsi 0 dht findpeer "$HASH" 2>err_findpeer &&
    test_must_fail ipfsi 0 dht put "/ipns/$PEERID_2" "get_result" 2>err_put &&
    test_should_contain "this command must be run in online mode" err_findprovs &&
    test_should_contain "this command must be run in online mode" err_findpeer &&
    test_should_contain "this command must be run in online mode" err_put
  '
}

test_dht
test_dht --enable-pubsub-experiment --enable-namesys-pubsub

test_done
