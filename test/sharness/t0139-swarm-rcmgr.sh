#!/usr/bin/env bash
#
test_description="Test ipfs swarm ResourceMgr config and commands"

. lib/test-lib.sh

test_init_ipfs

test_expect_success 'Disable resource manager' '
  ipfs config --bool Swarm.ResourceMgr.Enabled false
'

# test correct behavior when resource manager is disabled
test_launch_ipfs_daemon

test_expect_success 'Swarm limit should fail since RM is disabled' '
  test_expect_code 1 ipfs swarm limit system 2> actual &&
  test_should_contain "missing ResourceMgr" actual
'

test_expect_success 'Swarm stats should fail since RM is disabled' '
  test_expect_code 1 ipfs swarm stats all 2> actual &&
  test_should_contain "missing ResourceMgr" actual
'

test_kill_ipfs_daemon

test_expect_success 'Enable resource manager' '
  ipfs config --bool Swarm.ResourceMgr.Enabled true
'

# swarm limit|stats should fail in offline mode

test_expect_success 'disconnected: swarm limit requires running daemon' '
  test_expect_code 1 ipfs swarm limit system 2> actual &&
  test_should_contain "missing ResourceMgr" actual
'
test_expect_success 'disconnected: swarm stats requires running daemon' '
  test_expect_code 1 ipfs swarm stats all 2> actual &&
  test_should_contain "missing ResourceMgr" actual
'

# test sanity scaling
test_expect_success 'set very high connmgr highwater' '
  ipfs config --json Swarm.ConnMgr.HighWater 1000
'

test_launch_ipfs_daemon

test_expect_success 'conns and streams are above 2000' '
  ipfs swarm limit system --enc=json | tee json &&
  [ "$(jq -r .ConnsInbound < json)" -ge 2000 ] &&
  [ "$(jq -r .StreamsInbound < json)" -ge 2000 ]
'

test_kill_ipfs_daemon

test_expect_success 'set previous connmgr highwater' '
  ipfs config --json Swarm.ConnMgr.HighWater 96
'

test_launch_ipfs_daemon

test_expect_success 'conns and streams are above 800' '
  ipfs swarm limit system --enc=json | tee json &&
  [ "$(jq -r .ConnsInbound < json)" -ge 800 ] &&
  [ "$(jq -r .StreamsInbound < json)" -ge 800 ]
'

# swarm limit|stats should succeed in online mode by default
# because Resource Manager is opt-out

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
test_expect_success 'ResourceMgr enabled: swarm limit reset' '
  ipfs swarm limit system --reset --enc=json 2> reset &&
  ipfs swarm limit system --enc=json 2> actual &&
  test_cmp reset actual
'

test_expect_success 'Swarm stats system with filter should fail' '
  test_expect_code 1 ipfs swarm stats system --min-used-limit-perc=99 2> actual &&
  test_should_contain "Error: \"min-used-limit-perc\" can only be used when scope is \"all\"" actual
'

test_expect_success 'ResourceMgr enabled: swarm limit reset on map values' '
  ipfs swarm limit peer:12D3KooWL7i1T9VSPeF8AgQApbyM51GNKZsYPvNvL347aMDmvNzG --reset --enc=json 2> reset &&
  ipfs swarm limit peer:12D3KooWL7i1T9VSPeF8AgQApbyM51GNKZsYPvNvL347aMDmvNzG --enc=json 2> actual &&
  test_cmp reset actual
'

test_expect_success 'ResourceMgr enabled: scope is required using reset flag' '
  test_expect_code 1 ipfs swarm limit --reset 2> actual &&
  test_should_contain "Error: argument \"scope\" is required" actual
'

test_expect_success 'connected: swarm stats all working properly' '
  test_expect_code 0 ipfs swarm stats all
'

# every scope has the same fields, so we only inspect System
test_expect_success 'ResourceMgr enabled: swarm stats' '
  ipfs swarm stats all --enc=json | tee json &&
  jq -e .System.Memory < json &&
  jq -e .System.FD < json &&
  jq -e .System.Conns < json &&
  jq -e .System.ConnsInbound < json &&
  jq -e .System.ConnsOutbound < json &&
  jq -e .System.Streams < json &&
  jq -e .System.StreamsInbound < json &&
  jq -e .System.StreamsOutbound < json &&
  jq -e .Transient.Memory < json
'

# shut down the daemon, set a limit in the config, and verify that it's applied
test_kill_ipfs_daemon

test_expect_success "Set system conns limit while daemon is not running" "
  ipfs config --json Swarm.ResourceMgr.Limits.System.Conns 99999
"

test_expect_success "Set an invalid limit, which should result in a failure" "
  test_expect_code 1 ipfs config --json Swarm.ResourceMgr.Limits.System.Conns 'asdf' 2> actual &&
  test_should_contain 'failed to unmarshal' actual
"

test_launch_ipfs_daemon

test_expect_success 'Ensure the new system conns limit is applied' '
  ipfs swarm limit system --enc=json | tee json &&
  jq -e ".Conns == 99999" < json
'

test_expect_success 'Set system memory limit while the daemon is running' '
  ipfs swarm limit system | jq ".Memory = 99998" > system.json &&
  ipfs swarm limit system system.json
'

test_expect_success 'The new system limits were written to the config' '
  jq -e ".Swarm.ResourceMgr.Limits.System.Memory == 99998" < "$IPFS_PATH/config"
'

test_expect_success 'The new system limits are in the swarm limit output' '
  ipfs swarm limit system --enc=json | jq -e ".Memory == 99998"
'

# now test all the other scopes
test_expect_success 'Set limit on transient scope' '
  ipfs swarm limit transient | jq ".Memory = 88888" > transient.json &&
  ipfs swarm limit transient transient.json &&
  jq -e ".Swarm.ResourceMgr.Limits.Transient.Memory == 88888" < "$IPFS_PATH/config" &&
  ipfs swarm limit transient --enc=json | tee limits &&
  jq -e ".Memory == 88888" < limits
'

test_expect_success 'Set limit on service scope' '
  ipfs swarm limit svc:foo | jq ".Memory = 77777" > service-foo.json &&
  ipfs swarm limit svc:foo service-foo.json --enc=json &&
  jq -e ".Swarm.ResourceMgr.Limits.Service.foo.Memory == 77777" < "$IPFS_PATH/config" &&
  ipfs swarm limit svc:foo --enc=json | tee limits &&
  jq -e ".Memory == 77777" < limits
'

test_expect_success 'Set limit on protocol scope' '
  ipfs swarm limit proto:foo | jq ".Memory = 66666" > proto-foo.json &&
  ipfs swarm limit proto:foo proto-foo.json --enc=json &&
  jq -e ".Swarm.ResourceMgr.Limits.Protocol.foo.Memory == 66666" < "$IPFS_PATH/config" &&
  ipfs swarm limit proto:foo --enc=json | tee limits &&
  jq -e ".Memory == 66666" < limits
'

# any valid peer id
PEER_ID=QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN

test_expect_success 'Set limit on peer scope' '
  ipfs swarm limit peer:$PEER_ID | jq ".Memory = 66666" > peer-$PEER_ID.json &&
  ipfs swarm limit peer:$PEER_ID peer-$PEER_ID.json --enc=json &&
  jq -e ".Swarm.ResourceMgr.Limits.Peer.${PEER_ID}.Memory == 66666" < "$IPFS_PATH/config" &&
  ipfs swarm limit peer:$PEER_ID --enc=json | tee limits &&
  jq -e ".Memory == 66666" < limits
'

test_expect_success 'Get limit for peer scope with an invalid peer ID' '
  test_expect_code 1 ipfs swarm limit peer:foo 2> actual &&
  test_should_contain "invalid peer ID" actual
'

test_expect_success 'Set limit for peer scope with an invalid peer ID' '
  echo "{\"Memory\": 99}" > invalid-peer-id.json &&
  test_expect_code 1 ipfs swarm limit peer:foo invalid-peer-id.json 2> actual &&
  test_should_contain "invalid peer ID" actual
'

test_kill_ipfs_daemon

## Test allowlist

test_expect_success 'init iptb' '
  iptb testbed create -type localipfs -count 3 -init
'

test_expect_success 'peer ids' '
  PEERID_0=$(iptb attr get 0 id) &&
  PEERID_1=$(iptb attr get 1 id) &&
  PEERID_2=$(iptb attr get 2 id)
'

#enable resource manager
test_expect_success 'enable RCMGR' '
  ipfsi 0 config --bool Swarm.ResourceMgr.Enabled true &&
  ipfsi 0 config --json Swarm.ResourceMgr.Allowlist "[\"/ip4/0.0.0.0/ipcidr/0/p2p/$PEERID_2\"]"
'

test_expect_success 'start nodes' '
  iptb start -wait [0-2]
'

test_expect_success "change system limits on node 0" '
 ipfsi 0 swarm limit system | jq ". + {Conns: 0,ConnsInbound: 0, ConnsOutbound: 0}" > system.json &&
 ipfsi 0 swarm limit system system.json
'

test_expect_success "node 0 fails to connect to 1" '
  test_expect_code 1 iptb connect 0 1
'

test_expect_success "node 0 connects to 2 because it's allowlisted" '
  iptb connect 0 2
'

test_expect_success "node 0 fails to ping 1" '
  test_expect_code 1 ipfsi 0 ping -n2 -- "$PEERID_1" 2> actual &&
  test_should_contain "Error: ping failed" actual
'

test_expect_success "node 1 can ping 2" '
  ipfsi 0 ping -n2 -- "$PEERID_2"
'

test_expect_success 'stop iptb' '
  iptb stop 0 &&
  iptb stop 1 &&
  iptb stop 2
'

## Test daemon refuse to start if connmgr.highwater < ressources inbound

test_expect_success "node refuse to start if Swarm.ResourceMgr.Limits.System.Conns <= Swarm.ConnMgr.HighWater" '
  ipfs config --json Swarm.ResourceMgr.Limits.System.Conns 128 &&
  ipfs config --json Swarm.ConnMgr.HighWater 128 &&
  ipfs config --json Swarm.ConnMgr.LowWater 64 &&
  test_expect_code 1 ipfs daemon &&
  ipfs config --json Swarm.ResourceMgr.Limits.System.Conns 256
'

test_expect_success "node refuse to start if Swarm.ResourceMgr.Limits.System.ConnsInbound <= Swarm.ConnMgr.HighWater" '
  ipfs config --json Swarm.ResourceMgr.Limits.System.ConnsInbound 128 &&
  test_expect_code 1 ipfs daemon &&
  ipfs config --json Swarm.ResourceMgr.Limits.System.ConnsInbound 256
'

test_expect_success "node refuse to start if Swarm.ResourceMgr.Limits.System.Streams <= Swarm.ConnMgr.HighWater" '
  ipfs config --json Swarm.ResourceMgr.Limits.System.Streams 128 &&
  test_expect_code 1 ipfs daemon &&
  ipfs config --json Swarm.ResourceMgr.Limits.System.Streams 256
'

test_expect_success "node refuse to start if Swarm.ResourceMgr.Limits.System.StreamsInbound <= Swarm.ConnMgr.HighWater" '
  ipfs config --json Swarm.ResourceMgr.Limits.System.StreamsInbound 128 &&
  test_expect_code 1 ipfs daemon &&
  ipfs config --json Swarm.ResourceMgr.Limits.System.StreamsInbound 256
'

test_done
