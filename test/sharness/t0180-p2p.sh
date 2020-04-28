#!/usr/bin/env bash

test_description="Test experimental p2p commands"

. lib/test-lib.sh

# start iptb + wait for peering
test_expect_success 'init iptb' '
  iptb testbed create -type localipfs --count 3 --init
'

test_expect_success 'generate test data' '
  echo "ABCDEF" > test0.bin &&
  echo "012345" > test1.bin
'

startup_cluster 3

test_expect_success 'peer ids' '
  PEERID_0=$(iptb attr get 0 id) &&
  PEERID_1=$(iptb attr get 1 id)
'
check_test_ports() {
  test_expect_success "test ports are closed" '
    (! (netstat -aln | grep "LISTEN" | grep -E "[.:]10101 ")) &&
    (! (netstat -aln | grep "LISTEN" | grep -E "[.:]10102 ")) &&
    (! (netstat -aln | grep "LISTEN" | grep -E "[.:]10103 ")) &&
    (! (netstat -aln | grep "LISTEN" | grep -E "[.:]10104 "))
  '
}
check_test_ports

test_expect_success 'fail without config option being enabled' '
  test_must_fail ipfsi 0 p2p stream ls
'

test_expect_success "enable filestore config setting" '
  ipfsi 0 config --json Experimental.Libp2pStreamMounting true
  ipfsi 1 config --json Experimental.Libp2pStreamMounting true
  ipfsi 2 config --json Experimental.Libp2pStreamMounting true
'

test_expect_success 'start p2p listener' '
  ipfsi 0 p2p listen /x/p2p-test /ip4/127.0.0.1/tcp/10101 2>&1 > listener-stdouterr.log
'

test_expect_success 'cannot re-register p2p listener' '
  test_must_fail ipfsi 0 p2p listen /x/p2p-test /ip4/127.0.0.1/tcp/10103 2>&1 > listener-stdouterr.log
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

test_expect_success 'S->C(/p2p/peerID) Setup client side' '
  ipfsi 1 p2p forward /x/p2p-test /ip4/127.0.0.1/tcp/10102 /p2p/${PEERID_0} 2>&1 > dialer-stdouterr.log
'

test_expect_success 'S->C Setup(dnsaddr/addr/p2p/peerID) client side' '
  ipfsi 1 p2p forward /x/p2p-test /ip4/127.0.0.1/tcp/10103 /dnsaddr/bootstrap.libp2p.io/p2p/${PEERID_0}  2>&1 > dialer-stdouterr.log
'

test_expect_success 'S->C Setup(dnsaddr/addr) client side' '
  ipfsi 1 p2p forward /x/p2p-test /ip4/127.0.0.1/tcp/10104 /dnsaddr/cluster.ipfs.io 2>&1 > dialer-stdouterr.log
'


test_expect_success 'S->C Output is empty' '
  test_must_be_empty dialer-stdouterr.log
'

test_expect_success "'ipfs p2p ls | grep' succeeds" '
  ipfsi 1 p2p ls | grep "/x/p2p-test /ip4/127.0.0.1/tcp/10104"
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
  ipfsi 1 p2p close -p /x/p2p-test
'

check_test_ports

# Client to server communications

test_expect_success 'C->S Spawn receiving server' '
  ma-pipe-unidir --listen --pidFile=listener.pid recv /ip4/127.0.0.1/tcp/10101 > server.out &

  test_wait_for_file 30 100ms listener.pid &&
  kill -0 $(cat listener.pid)
'

test_expect_success 'C->S Setup client side' '
  ipfsi 1 p2p forward /x/p2p-test /ip4/127.0.0.1/tcp/10102 /p2p/${PEERID_0} 2>&1 > dialer-stdouterr.log
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
  ipfsi 1 p2p close -p /x/p2p-test
'

check_test_ports

# Checking port

test_expect_success "cannot accept 0 port in 'ipfs p2p listen'" '
  test_must_fail ipfsi 2 p2p listen /x/p2p-test/0 /ip4/127.0.0.1/tcp/0
'

test_expect_success "'ipfs p2p forward' accept 0 port" '
  ipfsi 2 p2p forward /x/p2p-test/0 /ip4/127.0.0.1/tcp/0 /p2p/$PEERID_0
'

test_expect_success "'ipfs p2p ls' output looks good" '
  echo "true" > forward_0_expected &&
  ipfsi 2 p2p ls | awk '\''{print $2}'\'' | sed "s/.*\///" | awk -F: '\''{if($1>0)print"true"}'\''  > forward_0_actual &&
  ipfsi 2 p2p close -p /x/p2p-test/0 &&
  test_cmp forward_0_expected forward_0_actual
'

# Listing streams

test_expect_success "'ipfs p2p ls' succeeds" '
  echo "/x/p2p-test /p2p/$PEERID_0 /ip4/127.0.0.1/tcp/10101" > expected &&
  ipfsi 0 p2p ls > actual
'

test_expect_success "'ipfs p2p ls' output looks good" '
  test_cmp expected actual
'

test_expect_success "Cannot re-register app handler" '
  test_must_fail ipfsi 0 p2p listen /x/p2p-test /ip4/127.0.0.1/tcp/10101
'

test_expect_success "'ipfs p2p stream ls' output is empty" '
  ipfsi 0 p2p stream ls > actual &&
  test_must_be_empty actual
'

check_test_ports

test_expect_success "Setup: Idle stream" '
  ma-pipe-unidir --listen --pidFile=listener.pid recv /ip4/127.0.0.1/tcp/10101 &

  ipfsi 1 p2p forward /x/p2p-test /ip4/127.0.0.1/tcp/10102 /p2p/$PEERID_0 &&
  ma-pipe-unidir --pidFile=client.pid recv /ip4/127.0.0.1/tcp/10102 &

  test_wait_for_file 30 100ms listener.pid &&
  test_wait_for_file 30 100ms client.pid &&
  kill -0 $(cat listener.pid) && kill -0 $(cat client.pid)
'

test_expect_success "'ipfs p2p stream ls' succeeds" '
  echo "3 /x/p2p-test /p2p/$PEERID_1 /ip4/127.0.0.1/tcp/10101" > expected
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
  ipfsi 0 p2p close -p /x/p2p-test &&
  ipfsi 0 p2p ls > actual &&
  test_must_be_empty actual
'

test_expect_success "'ipfs p2p close' closes local handler" '
  ipfsi 1 p2p close -p /x/p2p-test &&
  ipfsi 1 p2p ls > actual &&
  test_must_be_empty actual
'

check_test_ports

test_expect_success "Setup: Idle stream(2)" '
  ma-pipe-unidir --listen --pidFile=listener.pid recv /ip4/127.0.0.1/tcp/10101 &

  ipfsi 0 p2p listen /x/p2p-test2 /ip4/127.0.0.1/tcp/10101 2>&1 > listener-stdouterr.log &&
  ipfsi 1 p2p forward /x/p2p-test2 /ip4/127.0.0.1/tcp/10102 /p2p/$PEERID_0 2>&1 > dialer-stdouterr.log &&
  ma-pipe-unidir --pidFile=client.pid recv /ip4/127.0.0.1/tcp/10102 &

  test_wait_for_file 30 100ms listener.pid &&
  test_wait_for_file 30 100ms client.pid &&
  kill -0 $(cat listener.pid) && kill -0 $(cat client.pid)
'

test_expect_success "'ipfs p2p stream ls' succeeds(2)" '
  echo "4 /x/p2p-test2 /p2p/$PEERID_1 /ip4/127.0.0.1/tcp/10101" > expected
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
  ipfsi 0 p2p listen /x/1234 /ip4/127.0.0.1/tcp/10101 &&
  ipfsi 0 p2p close -p /x/1234 &&
  ipfsi 0 p2p ls > actual &&
  test_must_be_empty actual
'

test_expect_success "'ipfs p2p close' closes by target addr" '
  ipfsi 0 p2p listen /x/p2p-test /ip4/127.0.0.1/tcp/10101 &&
  ipfsi 0 p2p close -t /ip4/127.0.0.1/tcp/10101 &&
  ipfsi 0 p2p ls > actual &&
  test_must_be_empty actual
'

test_expect_success "'ipfs p2p close' closes right listeners" '
  ipfsi 0 p2p listen /x/p2p-test /ip4/127.0.0.1/tcp/10101 &&
  ipfsi 0 p2p forward /x/p2p-test /ip4/127.0.0.1/tcp/10101 /p2p/$PEERID_1 &&
  echo "/x/p2p-test /p2p/$PEERID_0 /ip4/127.0.0.1/tcp/10101" > expected &&

  ipfsi 0 p2p close -l /ip4/127.0.0.1/tcp/10101 &&
  ipfsi 0 p2p ls > actual &&
  test_cmp expected actual
'

check_test_ports

test_expect_success "'ipfs p2p close' closes by listen addr" '
  ipfsi 0 p2p close -l /p2p/$PEERID_0 &&
  ipfsi 0 p2p ls > actual &&
  test_must_be_empty actual
'

# Peer reporting

test_expect_success 'start p2p listener reporting peer' '
  ipfsi 0 p2p listen /x/p2p-test /ip4/127.0.0.1/tcp/10101 --report-peer-id 2>&1 > listener-stdouterr.log
'

test_expect_success 'C->S Spawn receiving server' '
  ma-pipe-unidir --listen --pidFile=listener.pid recv /ip4/127.0.0.1/tcp/10101 > server.out &

  test_wait_for_file 30 100ms listener.pid &&
  kill -0 $(cat listener.pid)
'

test_expect_success 'C->S Setup client side' '
  ipfsi 1 p2p forward /x/p2p-test /ip4/127.0.0.1/tcp/10102 /p2p/${PEERID_0} 2>&1 > dialer-stdouterr.log
'

test_expect_success 'C->S Connect and receive data' '
  ma-pipe-unidir send /ip4/127.0.0.1/tcp/10102 < test1.bin
'

test_expect_success 'C->S Ensure server finished' '
  go-sleep 250ms &&
  test ! -f listener.pid
'

test_expect_success 'C->S Output looks good' '
  echo ${PEERID_1} > expected &&
  cat test1.bin >> expected &&
  test_cmp server.out expected
'

test_expect_success 'C->S Close listeners' '
  ipfsi 1 p2p close -p /x/p2p-test &&
  ipfsi 0 p2p close -p /x/p2p-test &&

  ipfsi 0 p2p ls > actual &&
  test_must_be_empty actual &&

  ipfsi 1 p2p ls > actual &&
  test_must_be_empty actual
'

test_expect_success "non /x/ scoped protocols are not allowed" '
  test_must_fail ipfsi 0 p2p listen /its/not/a/x/path /ip4/127.0.0.1/tcp/10101 2> actual &&
  echo "Error: protocol name must be within '"'"'/x/'"'"' namespace" > expected
  test_cmp expected actual
'

check_test_ports

test_expect_success 'start p2p listener on custom proto' '
  ipfsi 0 p2p listen --allow-custom-protocol /p2p-test /ip4/127.0.0.1/tcp/10101 2>&1 > listener-stdouterr.log &&
  test_must_be_empty listener-stdouterr.log
'

spawn_sending_server

test_expect_success 'S->C Setup client side (custom proto)' '
  ipfsi 1 p2p forward --allow-custom-protocol /p2p-test /ip4/127.0.0.1/tcp/10102 /p2p/${PEERID_0} 2>&1 > dialer-stdouterr.log
'

test_server_to_client

test_expect_success 'C->S Close local listener' '
  ipfsi 1 p2p close -p /p2p-test
  ipfsi 1 p2p ls > actual &&
  test_must_be_empty actual
'

check_test_ports

test_expect_success 'stop iptb' '
  iptb stop
'

check_test_ports

test_done

