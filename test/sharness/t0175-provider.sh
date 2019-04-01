#!/usr/bin/env bash

test_description="Test reprovider"

. lib/test-lib.sh

NUM_NODES=2

init_cluster ${NUM_NODES}

start_node 0
start_node 1

has_no_peers 0
has_no_peers 1
connect_peers 0 1
has_peer 0 $node1
has_peer 1 $node0

add_data_to_node 0

stop_node 0

find_provs 1 $hash0 $node0

stop_node 1

test_done
