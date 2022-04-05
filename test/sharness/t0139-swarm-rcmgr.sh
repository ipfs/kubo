#!/usr/bin/env bash
#
test_description="Test ipfs swarm ResourceMgr config and commands"

. lib/test-lib.sh

test_init_ipfs

# swarm limit|stats should fail in offline mode

test_expect_success 'disconnected: swarm limit requires running daemon' '
    test_expect_code 1 ipfs swarm limit system 2> actual &&
    test_should_contain "missing ResourceMgr" actual
'
test_expect_success 'disconnected: swarm stats requires running daemon' '
    test_expect_code 1 ipfs swarm stats all 2> actual &&
    test_should_contain "missing ResourceMgr" actual
'

# swarm limit|stats should fail in online mode by default
# because Resource Manager is opt-in for now
test_launch_ipfs_daemon

test_expect_success 'ResourceMgr disabled by default: swarm limit requires Swarm.ResourceMgr.Enabled' '
    test_expect_code 1 ipfs swarm limit system 2> actual &&
    test_should_contain "missing ResourceMgr" actual
'
test_expect_success 'ResourceMgr disabled by default: swarm stats requires Swarm.ResourceMgr.Enabled' '
    test_expect_code 1 ipfs swarm stats all 2> actual &&
    test_should_contain "missing ResourceMgr" actual
'

# swarm limit|stat should work when Swarm.ResourceMgr.Enabled
test_kill_ipfs_daemon
test_expect_success "test_config_set succeeds" "
  ipfs config --json Swarm.ResourceMgr.Enabled true
"
test_launch_ipfs_daemon

# every scope has the same fields, so we only inspect System
test_expect_success 'ResourceMgr enabled: swarm limit' '
    ipfs swarm limit system --enc=json | tee json &&
    jq -e .Conns < json &&
    jq -e .ConnsInbound < json &&
    jq -e .ConnsOutbound < json &&
    jq -e .FD < json &&
    jq -e .Memory < json &&
    jq -e .Streams < json &&
    jq -e .StreamsInbound < json &&
    jq -e .StreamsOutbound < json
'

# every scope has the same fields, so we only inspect System
test_expect_success 'ResourceMgr enabled: swarm stats' '
    ipfs swarm stats all --enc=json | tee json &&
    jq -e .System.Memory < json &&
    jq -e .System.NumConnsInbound < json &&
    jq -e .System.NumConnsOutbound < json &&
    jq -e .System.NumFD < json &&
    jq -e .System.NumStreamsInbound < json &&
    jq -e .System.NumStreamsOutbound < json &&
    jq -e .Transient.Memory < json
'

test_kill_ipfs_daemon
test_done
