#!/usr/bin/env bash

test_description="Test to make sure our identity information looks sane"

. lib/test-lib.sh

test_init_ipfs

test_id_compute_agent() {
    AGENT_VERSION="$(ipfs version --number)" || return 1
    AGENT_COMMIT="$(ipfs version --number --commit)" || return 1
    if test "$AGENT_COMMIT" = "$AGENT_VERSION"; then
        AGENT_COMMIT=""
    else
        AGENT_COMMIT="${AGENT_COMMIT##$AGENT_VERSION-}"
    fi
    echo "go-ipfs/$AGENT_VERSION/$AGENT_COMMIT"
}

test_expect_success "checking AgentVersion" '
  test_id_compute_agent > expected-agent-version &&
  ipfs id -f "<aver>\n" > actual-agent-version &&
  test_cmp expected-agent-version actual-agent-version
'

test_expect_success "checking ProtocolVersion" '
  echo "ipfs/0.1.0" > expected-protocol-version &&
  ipfs id -f "<pver>\n" > actual-protocol-version &&
  test_cmp expected-protocol-version actual-protocol-version
'

test_expect_success "checking ID" '
  ipfs config Identity.PeerID > expected-id &&
  ipfs id -f "<id>\n" > actual-id &&
  test_cmp expected-id actual-id
'

test_done
