#!/bin/sh
# Stub script for the deprecated 'ipfs/go-ipfs' Docker image.
# This informs users to switch to 'ipfs/kubo'.

cat >&2 <<'EOF'
ERROR: The name 'go-ipfs' is no longer used.

Please update your Docker scripts to use 'ipfs/kubo' instead of 'ipfs/go-ipfs'.

For example:
  docker pull ipfs/kubo:release

More information:
  - https://github.com/ipfs/kubo#docker
  - https://hub.docker.com/r/ipfs/kubo
  - https://docs.ipfs.tech/install/run-ipfs-inside-docker/

EOF

exit 1
