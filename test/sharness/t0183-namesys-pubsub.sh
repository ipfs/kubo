#!/usr/bin/env bash

test_description="Test IPNS pubsub"

. lib/test-lib.sh

# start iptb + wait for peering
NUM_NODES=5
test_expect_success 'init iptb' '
    iptb testbed create -type localipfs -count $NUM_NODES -init
'

startup_cluster $NUM_NODES --enable-namesys-pubsub

test_expect_success 'peer ids' '
    PEERID_0_BASE36=$(ipfsi 0 key list --ipns-base=base36 -l | grep self | head -n 1 | cut -d " " -f1) &&
    PEERID_0_B58MH=$(ipfsi 0 key list --ipns-base=b58mh -l | grep self | head -n 1 | cut -d " " -f1)
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

# These commands are *expected* to fail. We haven't published anything yet.
test_expect_success 'subscribe nodes to the publisher topic' '
    ipfsi 1 name resolve /ipns/$PEERID_0_BASE36 --timeout=1s;
    ipfsi 2 name resolve /ipns/$PEERID_0_BASE36 --timeout=1s;
    true
'

test_expect_success 'check subscriptions' '
    echo /ipns/$PEERID_0_BASE36 > expected_base36 &&
    echo /ipns/$PEERID_0_B58MH > expected_b58mh &&
    ipfsi 1 name pubsub subs > subs1 &&
    ipfsi 2 name pubsub subs > subs2 &&
    ipfsi 1 name pubsub subs --ipns-base=b58mh > subs1_b58mh &&
    ipfsi 2 name pubsub subs --ipns-base=b58mh > subs2_b58mh &&
    test_cmp expected_base36 subs1 &&
    test_cmp expected_base36 subs2 &&
    test_cmp expected_b58mh subs1_b58mh &&
    test_cmp expected_b58mh subs2_b58mh
'

test_expect_success 'add an object on publisher node' '
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
    ipfsi 1 name resolve /ipns/$PEERID_0_BASE36 > name1 &&
    ipfsi 2 name resolve /ipns/$PEERID_0_BASE36 > name2 &&
    test_cmp expected name1 &&
    test_cmp expected name2
'

test_expect_success 'cancel subscriptions to the publisher topic' '
    ipfsi 1 name pubsub cancel /ipns/$PEERID_0_BASE36 &&
    ipfsi 2 name pubsub cancel /ipns/$PEERID_0_BASE36
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
