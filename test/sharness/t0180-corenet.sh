#!/bin/sh

test_description="Test corenet command"

. lib/test-lib.sh

# start iptb + wait for peering
test_expect_success 'init iptb' '
  iptb init -n 2 --bootstrap=none --port=0
'

startup_cluster 2

test_expect_success 'peer ids' '
  PEERID_0=$(iptb get id 0) &&
  PEERID_1=$(iptb get id 1)
'

test_expect_success 'generate test data' '
  dd if=/dev/urandom of=corenet0.bin bs=100K count=1 &&
  dd if=/dev/urandom of=corenet1.bin bs=100K count=1
'

# netcat (nc) is needed for the following tests
test_expect_success "nc is available" '
  type nc >/dev/null
'

# Test sending data from server to client
test_expect_success 'start netcat listeners' '
  dd if=corenet0.bin | nc -l -v -p 10001 &
  NC_SERVER_PID=$!
  nc -l -v -p 10002 | dd of=client.out &
  NC_CLIENT_PID=$!
  kill -0 $NC_SERVER_PID $NC_CLIENT_PID
'

test_expect_success 'start ipfs listener' '
  ipfsi 0 exp corenet listen /ip4/127.0.0.1/tcp/10001 corenet-test &
  LISTENER_PID=$!
  kill -0 $LISTENER_PID
'

test_expect_success 'Dial for server to client' '
  ipfsi 1 exp corenet dial $PEERID_0 /ip4/127.0.0.1/tcp/10002 corenet-test
'


test_expect_success 'wait for server to client test' '
  wait $NC_SERVER_PID $NC_CLIENT_PID &&
  kill $LISTENER_PID
'

test_expect_success 'server to client output looks good' '
  test_cmp client.out corenet0.bin
'

# Test sending data from client to server
test_expect_success 'start netcat listeners' '
  nc -l -v -p 10001 | dd of=server.out &
  NC_SERVER_PID=$!
  dd if=corenet1.bin | nc -l -v -p 10002 &
  NC_CLIENT_PID=$!
  kill -0 $NC_SERVER_PID $NC_CLIENT_PID
'

test_expect_success 'start ipfs listener' '
  ipfsi 0 exp corenet listen /ip4/127.0.0.1/tcp/10001 corenet-test &
  LISTENER_PID=$!
  kill -0 $LISTENER_PID
'

test_expect_success 'Dial for client to server' '
  ipfsi 1 exp corenet dial $PEERID_0 /ip4/127.0.0.1/tcp/10002 corenet-test
'

test_expect_success 'wait for client to server test' '
  wait $NC_SERVER_PID $NC_CLIENT_PID &&
  kill $LISTENER_PID
'

test_expect_success 'client to server output looks good' '
  test_cmp server.out corenet1.bin
'

test_expect_success 'stop iptb' '
  iptb stop
'

test_done
