#!/usr/bin/env bash

test_description="Test experimental p2p commands"

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
check_test_ports() {
  test_expect_success "test ports are closed" '
    (! (netstat -lnp | grep "LISTEN" | grep ":10101 ")) &&
    (! (netstat -lnp | grep "LISTEN" | grep ":10102 "))
  '
}
check_test_ports

test_expect_success 'fail without config option being enabled' '
  test_must_fail ipfsi 0 p2p stream ls
'

test_expect_success "enable filestore config setting" '
  ipfsi 0 config --json Experimental.Libp2pStreamMounting true
  ipfsi 1 config --json Experimental.Libp2pStreamMounting true
'

test_expect_success 'start p2p listener' '
  ipfsi 0 p2p forward /p2p/p2p-test /ipfs /ip4/127.0.0.1/tcp/10101 2>&1 > listener-stdouterr.log
'

# Server to client communications

spawn_sending_server() {
  test_expect_success 'S->C Spawn sending server' '
    ma-pipe-unidir --listen --pidFile=listener.pid send /ip4/127.0.0.1/tcp/10101 < test0.bin &

    test_wait_for_file 30 100ms listener.pid &&
    kill -0 $(cat listener.pid)
  '
}

test_server_to_client() {
  test_expect_success 'S->C Connect and receive data' '
    ma-pipe-unidir recv /ip4/127.0.0.1/tcp/10102 > client.out
  '

  test_expect_success 'S->C Ensure server finished' '
    test ! -f listener.pid
  '

  test_expect_success 'S->C Output looks good' '
    test_cmp client.out test0.bin
  '
}

spawn_sending_server

test_expect_success 'S->C Setup client side' '
  ipfsi 1 p2p forward /p2p/p2p-test /ip4/127.0.0.1/tcp/10102 /ipfs/${PEERID_0} 2>&1 > dialer-stdouterr.log
'

test_server_to_client

test_expect_success 'S->C Connect with dead server' '
  ma-pipe-unidir recv /ip4/127.0.0.1/tcp/10102 > client.out
'

test_expect_success 'S->C Output is empty' '
  test_must_be_empty client.out
'

spawn_sending_server

test_server_to_client

test_expect_success 'S->C Close local listener' '
  ipfsi 1 p2p close -p /p2p/p2p-test
'

check_test_ports

# Client to server communications

test_expect_success 'C->S Spawn receiving server' '
  ma-pipe-unidir --listen --pidFile=listener.pid recv /ip4/127.0.0.1/tcp/10101 > server.out &

  test_wait_for_file 30 100ms listener.pid &&
  kill -0 $(cat listener.pid)
'

test_expect_success 'C->S Setup client side' '
  ipfsi 1 p2p forward /p2p/p2p-test /ip4/127.0.0.1/tcp/10102 /ipfs/${PEERID_0} 2>&1 > dialer-stdouterr.log
'

test_expect_success 'C->S Connect and receive data' '
  ma-pipe-unidir send /ip4/127.0.0.1/tcp/10102 < test1.bin
'

test_expect_success 'C->S Ensure server finished' '
  go-sleep 250ms &&
  test ! -f listener.pid
'

test_expect_success 'C->S Output looks good' '
  test_cmp server.out test1.bin
'

test_expect_success 'C->S Close local listener' '
  ipfsi 1 p2p close -p /p2p/p2p-test
'

check_test_ports

# Listing streams

test_expect_success "'ipfs p2p ls' succeeds" '
  echo "/p2p/p2p-test /ipfs /ip4/127.0.0.1/tcp/10101" > expected &&
  ipfsi 0 p2p ls > actual
'

test_expect_success "'ipfs p2p ls' output looks good" '
  test_cmp expected actual
'

test_expect_success "Cannot re-register app handler" '
  test_must_fail ipfsi 0 p2p forward /p2p/p2p-test /ipfs /ip4/127.0.0.1/tcp/10101
'

test_expect_success "'ipfs p2p stream ls' output is empty" '
  ipfsi 0 p2p stream ls > actual &&
  test_must_be_empty actual
'

test_expect_success "Setup: Idle stream" '
  ma-pipe-unidir --listen --pidFile=listener.pid recv /ip4/127.0.0.1/tcp/10101 &

  ipfsi 1 p2p forward /p2p/p2p-test /ip4/127.0.0.1/tcp/10102  /ipfs/$PEERID_0 2>&1 > dialer-stdouterr.log &&
  ma-pipe-unidir --pidFile=client.pid recv /ip4/127.0.0.1/tcp/10102 &

  test_wait_for_file 30 100ms listener.pid &&
  test_wait_for_file 30 100ms client.pid &&
  kill -0 $(cat listener.pid) && kill -0 $(cat client.pid)
'

test_expect_success "'ipfs p2p stream ls' succeeds" '
  echo "3 /p2p/p2p-test /ipfs/$PEERID_1 /ip4/127.0.0.1/tcp/10101" > expected
  ipfsi 0 p2p stream ls > actual
'

test_expect_success "'ipfs p2p stream ls' output looks good" '
  test_cmp expected actual
'

test_expect_success "'ipfs p2p stream close' closes stream" '
  ipfsi 0 p2p stream close 3 &&
  ipfsi 0 p2p stream ls > actual &&
  [ ! -f listener.pid ] && [ ! -f client.pid ] &&
  test_must_be_empty actual
'

test_expect_success "'ipfs p2p close' closes remote handler" '
  ipfsi 0 p2p close -p /p2p/p2p-test &&
  ipfsi 0 p2p ls > actual &&
  test_must_be_empty actual
'

test_expect_success "'ipfs p2p close' closes local handler" '
  ipfsi 1 p2p close -p /p2p/p2p-test &&
  ipfsi 1 p2p ls > actual &&
  test_must_be_empty actual
'

check_test_ports

test_expect_success "Setup: Idle stream(2)" '
  ma-pipe-unidir --listen --pidFile=listener.pid recv /ip4/127.0.0.1/tcp/10101 &

  ipfsi 0 p2p forward /p2p/p2p-test2 /ipfs /ip4/127.0.0.1/tcp/10101 2>&1 > listener-stdouterr.log &&
  ipfsi 1 p2p forward /p2p/p2p-test2 /ip4/127.0.0.1/tcp/10102 /ipfs/$PEERID_0 2>&1 > dialer-stdouterr.log &&
  ma-pipe-unidir --pidFile=client.pid recv /ip4/127.0.0.1/tcp/10102 &

  test_wait_for_file 30 100ms listener.pid &&
  test_wait_for_file 30 100ms client.pid &&
  kill -0 $(cat listener.pid) && kill -0 $(cat client.pid)
'

test_expect_success "'ipfs p2p stream ls' succeeds(2)" '
  echo "4 /p2p/p2p-test2 /ipfs/$PEERID_1 /ip4/127.0.0.1/tcp/10101" > expected
  ipfsi 0 p2p stream ls > actual
  test_cmp expected actual
'

test_expect_success "'ipfs p2p close -a' closes remote app handlers" '
  ipfsi 0 p2p close -a &&
  ipfsi 0 p2p ls > actual &&
  test_must_be_empty actual
'

test_expect_success "'ipfs p2p close -a' closes local app handlers" '
  ipfsi 1 p2p close -a &&
  ipfsi 1 p2p ls > actual &&
  test_must_be_empty actual
'

test_expect_success "'ipfs p2p stream close -a' closes streams" '
  ipfsi 0 p2p stream close -a &&
  ipfsi 0 p2p stream ls > actual &&
  [ ! -f listener.pid ] && [ ! -f client.pid ] &&
  test_must_be_empty actual
'

check_test_ports

test_expect_success "'ipfs p2p close' closes app numeric handlers" '
  ipfsi 0 p2p forward /p2p/1234 /ipfs /ip4/127.0.0.1/tcp/10101 &&
  ipfsi 0 p2p close -p /p2p/1234 &&
  ipfsi 0 p2p ls > actual &&
  test_must_be_empty actual
'

test_expect_success "non /p2p/ scoped protocols are not allowed" '
  test_must_fail ipfsi 0 p2p forward /its/not/a/p2p/path /ipfs /ip4/127.0.0.1/tcp/10101 2> actual &&
  echo "Error: protocol name must be within '"'"'/p2p/'"'"' namespace" > expected
  test_cmp expected actual
'

check_test_ports

test_expect_success 'stop iptb' '
  iptb stop
'

test_done

