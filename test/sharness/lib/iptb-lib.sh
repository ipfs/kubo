# iptb test framework
#
# Copyright (c) 2014, 2016 Jeromy Johnson, Christian Couder
# MIT Licensed; see the LICENSE file in this repository.

export IPTB_ROOT="$(pwd)/.iptb"

ipfsi() {
  dir="$1"
  shift
  IPFS_PATH="$IPTB_ROOT/testbeds/default/$dir" ipfs "$@"
}

check_has_connection() {
  node="$1"
  ipfsi "$node" swarm peers >"swarm_peers_$node" &&
  grep "p2p" "swarm_peers_$node" >/dev/null
}

iptb() {
    if ! command iptb "$@"; then
        case "$1" in
            start|stop|connect)
                test_fsh command iptb logs
                ;;
        esac
        return 1
    fi
}

startup_cluster() {
  num_nodes="$1"
  shift
  other_args="$@"
  bound=$(expr "$num_nodes" - 1)

  if test -n "$other_args"; then
    test_expect_success "start up nodes with additional args" "
      iptb start -wait [0-$bound] -- ${other_args[@]}
    "
  else
    test_expect_success "start up nodes" '
      iptb start -wait [0-$bound]
    '
  fi

  test_expect_success "connect nodes to each other" '
    iptb connect [1-$bound] 0
  '

  for i in $(test_seq 0 "$bound")
  do
    test_expect_success "node $i is connected" '
      check_has_connection "$i" ||
      test_fsh cat "swarm_peers_$i"
    '
  done
}

iptb_wait_stop() {
    while ! iptb run -- sh -c '! { test -e "$IPFS_PATH/repo.lock" && fuser -f "$IPFS_PATH/repo.lock" >/dev/null; }'; do
        go-sleep 10ms
    done
}
