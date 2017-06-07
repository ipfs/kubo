#!/bin/sh

test_description="Test experimental ptp commands"

. lib/test-lib.sh

# start iptb + wait for peering
test_expect_success 'init iptb' '
  iptb init -n 2 --bootstrap=none --port=0
'

test_expect_success 'generate test data' '
  echo "ABCDEF" > test0.bin &&
  echo "012345" > test1.bin
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
  ipfsi 0 ptp stream ls
'

test_expect_success "enable filestore config setting" '
  ipfsi 0 config --json Experimental.Libp2pStreamMounting true
  ipfsi 1 config --json Experimental.Libp2pStreamMounting true
'

test_expect_success 'start ptp listener' '
  ipfsi 0 ptp listener open ptp-test /ip4/127.0.0.1/tcp/10101 2>&1 > listener-stdouterr.log
'

test_expect_success 'Test server to client communications' '
  ma-pipe-unidir --listen send /ip4/127.0.0.1/tcp/10101 < test0.bin &
  SERVER_PID=$!

  ipfsi 1 ptp stream dial $PEERID_0 ptp-test /ip4/127.0.0.1/tcp/10102 2>&1 > dialer-stdouterr.log &&
  ma-pipe-unidir recv /ip4/127.0.0.1/tcp/10102 > client.out &&
  wait $SERVER_PID
'

test_expect_success 'Test client to server communications' '
  ma-pipe-unidir --listen recv /ip4/127.0.0.1/tcp/10101 > server.out &
  SERVER_PID=$!

  ipfsi 1 ptp stream dial $PEERID_0 ptp-test /ip4/127.0.0.1/tcp/10102 2>&1 > dialer-stdouterr.log &&
  ma-pipe-unidir send /ip4/127.0.0.1/tcp/10102 < test1.bin
  wait $SERVER_PID
'

test_expect_success 'server to client output looks good' '
  test_cmp client.out test0.bin
'

test_expect_success 'client to server output looks good' '
  test_cmp server.out test1.bin
'

test_expect_success "'ipfs listener ptp ls' succeeds" '
  echo "/ip4/127.0.0.1/tcp/10101 /ptp/ptp-test" > expected &&
  ipfsi 0 ptp listener ls > actual
'

test_expect_success "'ipfs ptp listener ls' output looks good" '
  test_cmp expected actual
'

test_expect_success "Cannot re-register app handler" '
  (! ipfsi 0 ptp listener open ptp-test /ip4/127.0.0.1/tcp/10101)
'

test_expect_success "'ipfs ptp stream ls' output is empty" '
  ipfsi 0 ptp stream ls > actual &&
  test_must_be_empty actual
'

test_expect_success "Setup: Idle stream" '
  ma-pipe-unidir --listen --pidFile=listener.pid recv /ip4/127.0.0.1/tcp/10101 &

  ipfsi 1 ptp stream dial $PEERID_0 ptp-test /ip4/127.0.0.1/tcp/10102 2>&1 > dialer-stdouterr.log &&
  ma-pipe-unidir --pidFile=client.pid recv /ip4/127.0.0.1/tcp/10102 &

  go-sleep 500ms &&
  kill -0 $(cat listener.pid) && kill -0 $(cat client.pid)
'

test_expect_success "'ipfs ptp stream ls' succeeds" '
  echo "2 /ptp/ptp-test /ip4/127.0.0.1/tcp/10101 $PEERID_1" > expected
  ipfsi 0 ptp stream ls > actual
'

test_expect_success "'ipfs ptp stream ls' output looks good" '
  test_cmp expected actual
'

test_expect_success "'ipfs ptp stream close' closes stream" '
  ipfsi 0 ptp stream close 2 &&
  ipfsi 0 ptp stream ls > actual &&
  [ ! -f listener.pid ] && [ ! -f client.pid ] &&
  test_must_be_empty actual
'

test_expect_success "'ipfs ptp listener close' closes app handler" '
  ipfsi 0 ptp listener close ptp-test &&
  ipfsi 0 ptp listener ls > actual &&
  test_must_be_empty actual
'

test_expect_success "Setup: Idle stream(2)" '
  ma-pipe-unidir --listen --pidFile=listener.pid recv /ip4/127.0.0.1/tcp/10101 &

  ipfsi 0 ptp listener open ptp-test2 /ip4/127.0.0.1/tcp/10101 2>&1 > listener-stdouterr.log &&
  ipfsi 1 ptp stream dial $PEERID_0 ptp-test2 /ip4/127.0.0.1/tcp/10102 2>&1 > dialer-stdouterr.log &&
  ma-pipe-unidir --pidFile=client.pid recv /ip4/127.0.0.1/tcp/10102 &

  go-sleep 500ms &&
  kill -0 $(cat listener.pid) && kill -0 $(cat client.pid)
'

test_expect_success "'ipfs ptp stream ls' succeeds(2)" '
  echo "3 /ptp/ptp-test2 /ip4/127.0.0.1/tcp/10101 $PEERID_1" > expected
  ipfsi 0 ptp stream ls > actual
  test_cmp expected actual
'

test_expect_success "'ipfs ptp listener close -a' closes app handlers" '
  ipfsi 0 ptp listener close -a &&
  ipfsi 0 ptp listener ls > actual &&
  test_must_be_empty actual
'

test_expect_success "'ipfs ptp stream close -a' closes streams" '
  ipfsi 0 ptp stream close -a &&
  ipfsi 0 ptp stream ls > actual &&
  [ ! -f listener.pid ] && [ ! -f client.pid ] &&
  test_must_be_empty actual
'

test_expect_success "'ipfs ptp listener close' closes app numeric handlers" '
  ipfsi 0 ptp listener open 1234 /ip4/127.0.0.1/tcp/10101 &&
  ipfsi 0 ptp listener close 1234 &&
  ipfsi 0 ptp listener ls > actual &&
  test_must_be_empty actual
'

test_expect_success 'stop iptb' '
  iptb stop
'

test_done

