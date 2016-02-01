# iptb test framework 
#
# Copyright (c) 2014, 2016 Jeromy Johnson, Christian Couder
# MIT Licensed; see the LICENSE file in this repository.

export IPTB_ROOT="`pwd`/.iptb"

ipfsi() {
	dir="$1"
	shift
	IPFS_PATH="$IPTB_ROOT/$dir" ipfs $@
}

check_has_connection() {
	node=$1
	ipfsi $node swarm peers | grep ipfs > /dev/null
}

