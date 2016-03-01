# iptb test framework
#
# Copyright (c) 2014, 2016 Jeromy Johnson, Christian Couder
# MIT Licensed; see the LICENSE file in this repository.

export IPTB_ROOT="$(pwd)/.iptb"

ipfsi() {
	dir="$1"
	shift
	IPFS_PATH="$IPTB_ROOT/$dir" ipfs "$@"
}

check_has_connection() {
	node="$1"
	ipfsi "$node" swarm peers >"swarm_peers_$node" &&
	grep "ipfs" "swarm_peers_$node" >/dev/null
}

startup_cluster() {
	num_nodes="$1"
	bound=$(expr "$num_nodes" - 1)

	test_expect_success "start up nodes" '
		iptb start
	'

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
