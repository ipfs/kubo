#!/usr/bin/env bash

# Download the tarball of the IPFS client (i.e., reference implemention) to the `tmp` directory
wget https://dist.ipfs.io/go-ipfs/v0.4.18/go-ipfs_v0.4.18_linux-amd64.tar.gz -O /tmp/ipfs.tar.gz

# Decompress, extract, and move `go-ipfs` to the `tmp` directory
tar -xvzf /tmp/ipfs.tar.gz -C /opt

# Run the `install.sh` file
cd /opt/go-ipfs && ./install.sh

