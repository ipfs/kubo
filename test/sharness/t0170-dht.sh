#!/bin/sh

test_description="Test dht command"

. lib/test-lib.sh

test_init_ipfs

# start iptb + wait for peering
NUM_NODES=5
test_expect_success 'init iptb' '
  iptb init -n $NUM_NODES -f --bootstrap=none --port=0 &&
  startup_cluster $NUM_NODES
'

PEERID_0=$(ipfsi 0 id --format="<id>") &&

# publish
#HASH=$(echo 'hello warld' | ipfsi 0 add -q)
#test_expect_success "can publish before mounting /ipns" '
#  ipfsi 0 name publish '$HASH'
#'

sleep 1

# ipfs dht findpeer <peerID>
test_expect_success 'query' '
  touch expected &&
  ipfsi 1 dht findpeer $PEERID_0 >actual &&
	egrep "/ip4/127.0.0.1/tcp/.*" actual >/dev/null &&
	egrep "/ip4/.*/tcp/.*" actual >>/dev/null ||
	test_fsh cat actual
'
  # PEERS=$(wc -l actual | cut -d '"'"' '"'"' -f 1) &&
  # test $PEERS -gt 0
# ipfs dht query <peerID>
# ipfs dht findprovs <key>
# ipfs dht get <key>
# ipfs dht put <key> <value>

echo 'stopping..'
iptb stop
echo 'stopped!'

test_done
