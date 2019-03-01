#!/usr/bin/env bash

ssh -o 'StrictHostKeyChecking no' root@$IPFS_NODE_1 'bash -s' < 'setup/installer.sh'
