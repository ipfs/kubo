#!/usr/bin/env bash

test_description="Test to make sure our identity information looks sane"

. lib/test-lib.sh

test_init_ipfs

test_id_compute_agent() {
    local AGENT_SUFFIX
    AGENT_SUFFIX=$1
    AGENT_VERSION="$(ipfs version --number)" || return 1
    AGENT_COMMIT="$(ipfs version --number --commit)" || return 1
    if test "$AGENT_COMMIT" = "$AGENT_VERSION"; then
        AGENT_COMMIT=""
    else
        AGENT_COMMIT="${AGENT_COMMIT##$AGENT_VERSION-}"
    fi
    AGENT_VERSION="kubo/$AGENT_VERSION/$AGENT_COMMIT"
    if test -n "$AGENT_SUFFIX"; then
      if test -n "$AGENT_COMMIT"; then
        AGENT_VERSION="$AGENT_VERSION/"
      fi
      AGENT_VERSION="$AGENT_VERSION$AGENT_SUFFIX"
    fi
    echo "$AGENT_VERSION"
}

test_expect_success "checking AgentVersion" '
  test_id_compute_agent > expected-agent-version &&
  ipfs id -f "<aver>\n" > actual-agent-version &&
  test_cmp expected-agent-version actual-agent-version
'

test_expect_success "checking ID of self" '
  ipfs config Identity.PeerID > expected-id &&
  ipfs id -f "<id>\n" > actual-id &&
  test_cmp expected-id actual-id
'

test_expect_success "checking and converting ID of a random peer while offline" '
  # Peer ID taken from `t0140-swarm.sh` test.
  echo k2k4r8ncs1yoluq95unsd7x2vfhgve0ncjoggwqx9vyh3vl8warrcp15 > expected-id &&
  ipfs id -f "<id>\n" --peerid-base base36 --offline QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N > actual-id &&
  test_cmp expected-id actual-id
'

# agent-version-suffix (local, offline)
test_launch_ipfs_daemon --agent-version-suffix=test-suffix
test_expect_success "checking AgentVersion with suffix (local)" '
  test_id_compute_agent test-suffix > expected-agent-version &&
  ipfs id -f "<aver>\n" > actual-agent-version &&
  test_cmp expected-agent-version actual-agent-version
'

# agent-version-suffix (over libp2p identify protocol)
iptb testbed create -type localipfs -count 2 -init
startup_cluster 2 --agent-version-suffix=test-suffix-identify
test_expect_success "checking AgentVersion with suffix (fetched via libp2p identify protocol)" '
  ipfsi 0 id -f "<aver>\n" > expected-identify-agent-version &&
  ipfsi 1 id "$(ipfsi 0 config Identity.PeerID)" -f "<aver>\n" > actual-libp2p-identify-agent-version &&
  test_cmp expected-identify-agent-version actual-libp2p-identify-agent-version
'
iptb stop

test_kill_ipfs_daemon

# Version.AgentSuffix overrides --agent-version-suffix (local, offline)
test_expect_success "setting Version.AgentSuffix in config" '
  ipfs config Version.AgentSuffix json-config-suffix
'
test_launch_ipfs_daemon --agent-version-suffix=ignored-cli-suffix
test_expect_success "checking AgentVersion with suffix set via JSON config" '
  test_id_compute_agent json-config-suffix > expected-agent-version &&
  ipfs id -f "<aver>\n" > actual-agent-version &&
  test_cmp expected-agent-version actual-agent-version
'
test_kill_ipfs_daemon

test_done
