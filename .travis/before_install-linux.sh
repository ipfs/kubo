#!/bin/bash
set -ev

sudo apt-get install -qq pkg-config fuse
sudo modprobe fuse
sudo chmod 666 /dev/fuse
sudo chown root:$USER /etc/fuse.conf
