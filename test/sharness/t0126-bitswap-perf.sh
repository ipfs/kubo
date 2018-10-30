#!/usr/bin/env bash
#
# Copyright (c) 2017 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test multiple ipfs nodes transferring files via bitswap"
timing_file="`basename "$0"`-timings.tmp"
node_count=8
file_size_kb=10000

. lib/test-lib.sh

time_expect_success() {
    { time {
    test_expect_success "$1" "$2";
    TIMEFORMAT=%5R;
    }; } 2>> $timing_file
}

check_file_fetch() {
  node=$1
  fhash=$2
  fname=$3

  echo -n "node$1-receive-elapsed-sec: " >> $timing_file
  time_expect_success "node$node can fetch file" '
    ipfsi $node cat $fhash > fetch_out$node
  '

  test_expect_success "file looks good" '
    test_cmp $fname fetch_out$node
  '

  echo -n "node$1-receive-duplicate_blocks: " >> $timing_file &&
  ipfsi $1 bitswap stat | sed -n -e 's/[[:space:]]dup blocks received: \([0-9].*\)$/\1/p' >> $timing_file
}

echo '' > $timing_file

test_expect_success "set up tcp testbed" '
  iptb init -n $node_count -p 0 -f --bootstrap=none
'

startup_cluster $node_count

# Clean out all the repos
for i in $(test_seq 0 $(expr $node_count - 1))
do
  test_expect_success "clean node $i repo before test" '
    ipfsi $i repo gc > /dev/null
  '
done

# Create a big'ish file
test_expect_success "add a file on node0" '
  random $(($file_size_kb * 1024)) > filea &&
  FILEA_HASH=$(ipfsi 0 add -q filea)
'

# Fetch the file with each node in succession (time each)
for i in $(test_seq 1 $(expr $node_count - 1))
do
  check_file_fetch $i $FILEA_HASH filea
  sleep 2
done

# shutdown
test_expect_success "shut down nodes" '
  iptb stop && iptb_wait_stop
'

cat $timing_file

test_done