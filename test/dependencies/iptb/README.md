# IPTB
iptb is a program used to manage a cluster of ipfs nodes locally on your
computer. It allows the creation of up to 1000 (limited by poor port choice)
nodes, and allows for various other setup options to be selected such as
different bootstrapping patterns. iptb makes testing networks in ipfs
easy!

### Commands:
- init 
	- creates and initializes 'n' repos
	- Options:
		- -n=[number of nodes]
		- -f : force overwriting of existing nodes
		- -bootstrap : select bootstrapping style for cluster choices: star, none
		- -mdns=[true||false] : defaults to false
		- -p=[start port] : port to start allocations from
- start 
	- starts up all testbed nodes
	- Options:
		- -wait : wait until daemons are fully initialized
- stop 
	- kills all testbed nodes
- restart
	- kills and then restarts all testbed nodes

- shell [n]
	- execs your shell with environment variables set as follows:
	    - IPFS_PATH - set to testbed node n's IPFS_PATH
	    - NODE[x] - set to the peer ID of node x

- get [attr] [n]
	- gets the specified attribute from then given node
		- available attributes: id

### Configuration
By default, iptb uses `$HOME/testbed` to store created nodes. This path is
configurable via the environment variables `IPTB_ROOT`. 



