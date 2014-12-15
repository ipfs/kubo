# must be connected to bootstrap node
ipfs bootstrap add /ip4/$BOOTSTRAP_PORT_4011_TCP_ADDR/tcp/$BOOTSTRAP_PORT_4011_TCP_PORT/QmNXuBh8HFsWq68Fid8dMbGNQTh7eG6hV9rr1fQyfmfomE

# wait for daemon to start/bootstrap
# alternatively use ipfs swarm connect
ipfs daemon &
sleep 3
echo $(ipfs id)
# TODO instead of bootrapping: ipfs swarm connect /ip4/$BOOTSTRAP_PORT_4011_TCP_ADDR/tcp/$BOOTSTRAP_PORT_4011_TCP_PORT/QmNXuBh8HFsWq68Fid8dMbGNQTh7eG6hV9rr1fQyfmfomE

# must mount this volume from data container
ipfs add -q /data/file > /data/id

echo added file. hash is $(cat /data/id)

# allow ample time for the client to pull the data
sleep 10000000
