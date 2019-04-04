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
  ipfsi "$node" swarm peers >"swarm_peers_$node"
  grep "ipfs" "swarm_peers_$node" >/dev/null
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
      iptb start -wait -- ${other_args[@]}
    "
  else
    test_expect_success "start up nodes" '
      iptb start -wait
    '
  fi

  test_expect_success "connect nodes to eachother" '
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

init_cluster() {
    test_expect_success "init iptb" "
        iptb testbed create -type localipfs -force -count $1 -init
    "

    for ((i=0; i<$1; i++));
    do
        test_expect_success "node id $i" "
            node$i=$(iptb attr get $i id)
        "
    done
}

start_node() {
    test_expect_success "start up node $1" "
        iptb start -wait $1
    "
}

stop_node() {
    test_expect_success "stop node $1" "
        iptb stop $1
    "
}

add_data_to_node() {
    test_expect_success "generate test object" "
        head -c 256 </dev/urandom >object$1
    "

    test_expect_success "add test object" "
        hash$1=$(ipfsi $1 add -q "object$1")
    "
}

connect_peers() {
    test_expect_success "connect node $1 to node $2" "
        iptb connect $1 $2
    "
}

not_find_provs() {
    test_expect_success "findprovs "$2" succeeds" "
        ipfsi $1 dht findprovs -n 1 "$2" > findprovs_$2
    "

    test_expect_success "findprovs $2 output is empty" "
        test_must_be_empty findprovs_$2
    "
}

find_provs() {
    test_expect_success "prepare expected succeeds" "
        echo $3 > expected$1
    "

    test_expect_success "findprovs "$2" succeeds" "
        ipfsi $1 dht findprovs -n 1 "$2" > findprovs_$2
    "

    test_expect_success "findprovs $2 output looks good" "
        test_cmp findprovs_$2 expected$1
    "
}

has_no_peers() {
    test_expect_success "get peers for node 0" "
        ipfsi $1 swarm peers >swarm_peers_$1
    "

    test_expect_success "swarm_peers_$1 is empty" "
        test_must_be_empty swarm_peers_$1
    "
}

has_peer() {
    test_expect_success "prepare expected succeeds" "
        echo $2 > expected$1
    "

    test_expect_success "get peers for node $1" "
        ipfsi $1 swarm peers >swarm_peers_$1
    "

    test_expect_success "swarm_peers_$1 contains $2" "
        cat swarm_peers_$1 | grep $2
    "
}

reprovide() {
    test_expect_success "reprovide" "
        ipfsi $1 provider reprovide
    "
}
