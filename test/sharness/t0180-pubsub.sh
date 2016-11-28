#!/bin/sh

test_description="Test dht command"

. lib/test-lib.sh

# start iptb + wait for peering
NUM_NODES=5
test_expect_success 'init iptb' '
  iptb init -n $NUM_NODES --bootstrap=none --port=0
'

startup_cluster $NUM_NODES --enable-pubsub-experiment

test_expect_success 'peer ids' '
  PEERID_0=$(iptb get id 0) &&
  PEERID_2=$(iptb get id 2)
'

# ipfs pubsub sub
test_expect_success 'pubsub' '
	echo "testOK" > expected &&
	touch empty &&
	mkfifo wait ||
	test_fsh echo init fail

	(
		ipfsi 0 pubsub sub --enc=ndpayload testTopic | 
			while read line; do
				echo $line > actual &&
				echo > done
				exit
			done
	) &

	ipfspid=$!

	sleep 1

	# publish something
	ipfsi 1 pubsub pub testTopic "testOK" &> pubErr &&

	# wait until `echo > wait` executed
	cat wait &&

	test_cmp pubErr empty &&
	test_cmp expected actual 
'

test_expect_success 'stop iptb' '
  iptb stop
'

test_done
