#Ipfs Testbed

##commands:

### init -n=[number of nodes]
creates and initializes 'n' repos

### start 
starts up all testbed nodes

### stop 
kills all testbed nodes

### restart
kills, then restarts all testbed nodes

### shell [n]
execs your shell with environment variables set as follows:
- IPFS_PATH - set to testbed node n's IPFS_PATH
- NODE[x] - set to the peer ID of node x
