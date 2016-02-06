#!/bin/bash
set -ev

sudo modprobe fuse
sudo chmod 666 /dev/fuse
sudo chown root:$USER /etc/fuse.conf
