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

# netcat (nc) is needed for the following tests
test_expect_success "nc is available" '
  type nc >/dev/null
'

test_expect_success 'start ipfs listener' '
  ipfsi 0 exp corenet listen corenet-test /ip4/127.0.0.1/tcp/10001 2>&1 > listener-stdouterr.log
'

test_expect_success 'Test server to client communications' '
  dd if=corenet0.bin | nc -l 127.0.0.1 10001 &
  NC_SERVER_PID=$!

  ipfsi 1 exp corenet dial $PEERID_0 corenet-test /ip4/127.0.0.1/tcp/10002 2>&1 > dialer-stdouterr.log &&
  nc -v 127.0.0.1 10002 | dd of=client.out &&

  wait $NC_SERVER_PID
'

test_expect_success 'Test server to client communications' '
  nc -l 127.0.0.1 10001 | dd of=server.out &
  NC_SERVER_PID=$!

  ipfsi 1 exp corenet dial $PEERID_0 corenet-test /ip4/127.0.0.1/tcp/10002 2>&1 > dialer-stdouterr.log &&
  dd of=corenet1.bin | nc -v 127.0.0.1 10002 &&

  wait $NC_SERVER_PID
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

