set -x
set -v

# The daemon needs to be running, otherwise the commands will compete on their
# lock of the entire repo (not the MFS root) and fail.
ipfs swarm addrs > /dev/null || (echo "daemon not running" && exit 1)

ipfs --version
ipfs files mkdir /test-lock/
ipfs files rm /test-lock/ -r
((echo "content" | ./cmd/ipfs/ipfs files write --create --parents --truncate --lock-time 3 /test-lock/file) && echo "ipfs write lock released" &)
ipfs repo gc
# FIXME: This is a flaky test just to manually check the lock in ipfs write is blocking the GC.
