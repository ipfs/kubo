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

test_must_fail 'fail without config option being enabled' '
  ipfsi 0 exp corenet ls
'

test_expect_success "enable filestore config setting" '
  ipfsi 0 config --json Experimental.Libp2pStreamMounting true
  ipfsi 1 config --json Experimental.Libp2pStreamMounting true
'

test_expect_success 'start corenet listener' '
  ipfsi 0 corenet listen corenet-test /ip4/127.0.0.1/tcp/10101 2>&1 > listener-stdouterr.log
'

test_expect_success 'Test server to client communications' '
  ma-pipe-unidir --listen send /ip4/127.0.0.1/tcp/10101 < corenet0.bin &
  SERVER_PID=$!

  ipfsi 1 corenet dial $PEERID_0 corenet-test /ip4/127.0.0.1/tcp/10102 2>&1 > dialer-stdouterr.log &&
  ma-pipe-unidir recv /ip4/127.0.0.1/tcp/10102 > client.out &&
  wait $SERVER_PID
'

test_expect_success 'Test client to server communications' '
  ma-pipe-unidir --listen recv /ip4/127.0.0.1/tcp/10101 > server.out &
  SERVER_PID=$!

  ipfsi 1 corenet dial $PEERID_0 corenet-test /ip4/127.0.0.1/tcp/10102 2>&1 > dialer-stdouterr.log &&
  ma-pipe-unidir send /ip4/127.0.0.1/tcp/10102 < corenet1.bin
  wait $SERVER_PID
'

test_expect_success 'server to client output looks good' '
  test_cmp client.out corenet0.bin
'

test_expect_success 'client to server output looks good' '
  test_cmp server.out corenet1.bin
'

test_expect_success "'ipfs corenet ls' succeeds" '
  echo "/ip4/127.0.0.1/tcp/10101 /app/corenet-test" > expected &&
  ipfsi 0 corenet ls > actual
'

test_expect_success "'ipfs corenet ls' output looks good" '
  test_cmp expected actual
'

test_expect_success "Cannot re-register app handler" '
  (! ipfsi 0 corenet listen corenet-test /ip4/127.0.0.1/tcp/10101)
'

test_expect_success "'ipfs corenet streams' output is empty" '
  ipfsi 0 corenet streams > actual &&
  test_must_be_empty actual
'

test_expect_success "Setup: Idle stream" '
  ma-pipe-unidir --listen --pidFile=listener.pid recv /ip4/127.0.0.1/tcp/10101 &

  ipfsi 1 corenet dial $PEERID_0 corenet-test /ip4/127.0.0.1/tcp/10102 2>&1 > dialer-stdouterr.log &&
  ma-pipe-unidir --pidFile=client.pid recv /ip4/127.0.0.1/tcp/10102 &

  go-sleep 500ms &&
  kill -0 $(cat listener.pid) && kill -0 $(cat client.pid)
'

test_expect_success "'ipfs corenet streams' succeeds" '
  echo "2 /app/corenet-test /ip4/127.0.0.1/tcp/10101 $PEERID_1" > expected
  ipfsi 0 corenet streams > actual
'

test_expect_success "'ipfs corenet streams' output looks good" '
  test_cmp expected actual
'

test_expect_success "'ipfs corenet close' closes stream" '
  ipfsi 0 corenet close 2 &&
  ipfsi 0 corenet streams > actual &&
  [ ! -f listener.pid ] && [ ! -f client.pid ] &&
  test_must_be_empty actual
'

test_expect_success "'ipfs corenet close' closes app handler" '
  ipfsi 0 corenet close corenet-test &&
  ipfsi 0 corenet ls > actual &&
  test_must_be_empty actual
'

test_expect_success "Setup: Idle stream(2)" '
  ma-pipe-unidir --listen --pidFile=listener.pid recv /ip4/127.0.0.1/tcp/10101 &

  ipfsi 0 corenet listen corenet-test2 /ip4/127.0.0.1/tcp/10101 2>&1 > listener-stdouterr.log &&
  ipfsi 1 corenet dial $PEERID_0 corenet-test2 /ip4/127.0.0.1/tcp/10102 2>&1 > dialer-stdouterr.log &&
  ma-pipe-unidir --pidFile=client.pid recv /ip4/127.0.0.1/tcp/10102 &

  go-sleep 500ms &&
  kill -0 $(cat listener.pid) && kill -0 $(cat client.pid)
'

test_expect_success "'ipfs corenet streams' succeeds(2)" '
  echo "3 /app/corenet-test2 /ip4/127.0.0.1/tcp/10101 $PEERID_1" > expected
  ipfsi 0 corenet streams > actual
  test_cmp expected actual
'

test_expect_success "'ipfs corenet close -a' closes streams and app handlers" '
  ipfsi 0 corenet close -a &&
  ipfsi 0 corenet streams > actual &&
  [ ! -f listener.pid ] && [ ! -f client.pid ] &&
  test_must_be_empty actual &&
  ipfsi 0 corenet ls > actual &&
  test_must_be_empty actual
'

test_expect_success 'stop iptb' '
  iptb stop
'

test_done

