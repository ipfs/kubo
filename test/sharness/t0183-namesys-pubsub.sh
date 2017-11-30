#!/bin/sh

test_description="Test IPNS pubsub"

. lib/test-lib.sh

# start iptb + wait for peering
NUM_NODES=5
test_expect_success 'init iptb' '
    iptb init -n $NUM_NODES --bootstrap=none --port=0
'

startup_cluster $NUM_NODES --enable-namesys-pubsub

test_expect_success 'peer ids' '
    PEERID_0=$(iptb get id 0)
'

test_expect_success 'check namesys pubsub state' '
    echo enabled > expected &&
    ipfsi 0 name pubsub state > state0 &&
    ipfsi 1 name pubsub state > state1 &&
    ipfsi 2 name pubsub state > state2 &&
    test_cmp expected state0 &&
    test_cmp expected state1 &&
    test_cmp expected state2
'

test_expect_success 'subscribe nodes to the publisher topic' '
    ipfsi 1 name resolve /ipns/$PEERID_0 &&
    ipfsi 2 name resolve /ipns/$PEERID_0
'

test_expect_success 'check subscriptions' '
    echo /ipns/$PEERID_0 > expected &&
    ipfsi 1 name pubsub subs > subs1 &&
    ipfsi 2 name pubsub subs > subs2 &&
    test_cmp expected subs1 &&
    test_cmp expected subs2
'

test_expect_success 'add an obect on publisher node' '
    echo "ipns is super fun" > file &&
    HASH_FILE=$(ipfsi 0 add -q file)
'

test_expect_success 'publish that object as an ipns entry' '
    ipfsi 0 name publish $HASH_FILE
'

test_expect_success 'wait for the flood' '
    sleep 1
'

test_expect_success 'resolve name in subscriber nodes' '
    echo "/ipfs/$HASH_FILE" > expected &&
    ipfsi 1 name resolve /ipns/$PEERID_0 > name1 &&
    ipfsi 2 name resolve /ipns/$PEERID_0 > name2 &&
    test_cmp expected name1 &&
    test_cmp expected name2
'

test_expect_success 'cancel subscriptions to the publisher topic' '
    ipfsi 1 name pubsub cancel /ipns/$PEERID_0 &&
    ipfsi 2 name pubsub cancel /ipns/$PEERID_0
'

test_expect_success 'check subscriptions' '
    rm -f expected && touch expected &&
    ipfsi 1 name pubsub subs > subs1 &&
    ipfsi 2 name pubsub subs > subs2 &&
    test_cmp expected subs1 &&
    test_cmp expected subs2
'

test_expect_success "shut down iptb" '
    iptb stop
'

test_done
