#!/bin/sh

test_description="Test experimental corenet commands"

. lib/test-lib.sh

# start iptb + wait for peering
test_expect_success 'init iptb' '
  iptb init -n 2 --bootstrap=none --port=0
'

test_expect_success 'generate test data' '
  echo "ABCDEF" > corenet0.bin &&
  echo "012345" > corenet1.bin
'

startup_cluster 2

test_expect_success 'peer ids' '
  PEERID_0=$(iptb get id 0) &&
  PEERID_1=$(iptb get id 1)
'

test_expect_success "test ports are closed" '
  (! (netstat -ln | grep "LISTEN" | grep ":10101 ")) &&
  (! (netstat -ln | grep "LISTEN" | grep ":10102 "))
'

test_expect_success 'start ipfs listener' '
  ipfsi 0 exp corenet listen corenet-test /ip4/127.0.0.1/tcp/10101 2>&1 > listener-stdouterr.log
'

test_expect_success 'Test server to client communications' '
  ma-pipe-unidir --listen send /ip4/127.0.0.1/tcp/10101 < corenet0.bin &
  SERVER_PID=$!

  ipfsi 1 exp corenet dial $PEERID_0 corenet-test /ip4/127.0.0.1/tcp/10102 2>&1 > dialer-stdouterr.log &&
  ma-pipe-unidir recv /ip4/127.0.0.1/tcp/10102 > client.out &&
  wait $SERVER_PID
'

test_expect_success 'Test client to server communications' '
  ma-pipe-unidir --listen recv /ip4/127.0.0.1/tcp/10101 > server.out &
  SERVER_PID=$!
  #sleep 0.5 &&

  ipfsi 1 exp corenet dial $PEERID_0 corenet-test /ip4/127.0.0.1/tcp/10102 2>&1 > dialer-stdouterr.log &&
  ma-pipe-unidir send /ip4/127.0.0.1/tcp/10102 < corenet1.bin
  wait $SERVER_PID
'

test_expect_success 'server to client output looks good' '
  test_cmp client.out corenet0.bin
'

test_expect_success 'client to server output looks good' '
  test_cmp server.out corenet1.bin
'

test_expect_success 'stop iptb' '
  iptb stop
'

test_done

