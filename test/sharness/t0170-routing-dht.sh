#!/usr/bin/env bash

# This file does the same tests as t0170-dht.sh but uses 'routing' commands instead
# (only exception is query, which lives only under dht)
test_description="Test routing command for DHT queries"

. lib/test-lib.sh

test_dht() {
  NUM_NODES=5

  test_expect_success 'init iptb' '
    rm -rf .iptb/ &&
    iptb testbed create -type localipfs -count $NUM_NODES -init
  '

  test_expect_success 'DHT-only routing' '
    iptb run -- ipfs config Routing.Type dht
  '

  startup_cluster $NUM_NODES $@

  test_expect_success 'peer ids' '
    PEERID_0=$(iptb attr get 0 id) &&
    PEERID_2=$(iptb attr get 2 id)
  '
  
  # ipfs routing findpeer <peerID>
  test_expect_success 'findpeer' '
    ipfsi 1 routing findpeer $PEERID_0 | sort >actual &&
    ipfsi 0 id -f "<addrs>" | cut -d / -f 1-5 | sort >expected &&
    test_cmp actual expected
  '
  
  # ipfs routing get <key>
  test_expect_success 'get with good keys works' '
    HASH="$(echo "hello world" | ipfsi 2 add -q)" &&
    ipfsi 2 name publish "/ipfs/$HASH" &&
    ipfsi 1 routing get "/ipns/$PEERID_2" >get_result
  '

  test_expect_success 'get with good keys contains the right value' '
    cat get_result | grep -aq "/ipfs/$HASH"
  '

  test_expect_success 'put round trips (#3124)' '
    ipfsi 0 routing put "/ipns/$PEERID_2" get_result | sort >putted &&
    [ -s putted ] ||
    test_fsh cat putted
  '
  
  test_expect_success 'put with bad keys fails (issue #5113)' '
    ipfsi 0 routing put "foo" <<<bar >putted
    ipfsi 0 routing put "/pk/foo" <<<bar >>putted
    ipfsi 0 routing put "/ipns/foo" <<<bar >>putted
    [ ! -s putted ] ||
    test_fsh cat putted
  '
  
  test_expect_success 'put with bad keys returns error (issue #4611)' '
    test_must_fail ipfsi 0 routing put "foo" <<<bar &&
    test_must_fail ipfsi 0 routing put "/pk/foo" <<<bar &&
    test_must_fail ipfsi 0 routing put "/ipns/foo" <<<bar
  '
  
  test_expect_success 'get with bad keys (issue #4611)' '
    test_must_fail ipfsi 0 routing get "foo" &&
    test_must_fail ipfsi 0 routing get "/pk/foo"
  '
  
  test_expect_success "add a ref so we can find providers for it" '
    echo "some stuff" > afile &&
    HASH=$(ipfsi 3 add -q afile)
  '
  
  # ipfs routing findprovs <key>
  test_expect_success 'findprovs' '
    ipfsi 4 routing findprovs $HASH > provs &&
    iptb attr get 3 id > expected &&
    test_cmp provs expected
  '
  
  
  # ipfs dht query <peerID>
  #
  # We test all nodes. 4 nodes should see the same peer ID, one node (the
  # closest) should see a different one.

  for i in $(test_seq 0 4); do
    test_expect_success "dht query from $i" '
      ipfsi "$i" dht query "$HASH" | head -1 >closest-$i
    '
  done

  test_expect_success "collecting results" '
    cat closest-* | sort | uniq -c | sed -e "s/ *\([0-9]\+\) .*/\1/g" | sort -g > actual &&
    echo 1 > expected &&
    echo 4 >> expected
  '

  test_expect_success "checking results" '
    test_cmp actual expected
  '

  test_expect_success 'stop iptb' '
    iptb stop
  '

  test_expect_success "dht commands fail when offline" '
    test_must_fail ipfsi 0 routing findprovs "$HASH" 2>err_findprovs &&
    test_must_fail ipfsi 0 routing findpeer "$HASH" 2>err_findpeer &&
    test_must_fail ipfsi 0 routing put "/ipns/$PEERID_2" "get_result" 2>err_put &&
    test_should_contain "this command must be run in online mode" err_findprovs &&
    test_should_contain "this command must be run in online mode" err_findpeer &&
    test_should_contain "this command must be run in online mode" err_put
  '
}

test_dht
test_dht --enable-pubsub-experiment --enable-namesys-pubsub

test_done
