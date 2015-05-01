#!/bin/bash

src_dir=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
plist=io.ipfs.ipfs-daemon.plist
dest_dir="$HOME/Library/LaunchAgents"
IPFS_PATH="${IPFS_PATH:-$HOME/.ipfs}"
escaped_ipfs_path=$(echo $IPFS_PATH|sed 's/\//\\\//g')

mkdir -p "$dest_dir"

sed 's/{{IPFS_PATH}}/'"$escaped_ipfs_path"'/g' \
  "$src_dir/$plist" \
  > "$dest_dir/$plist"

launchctl list | grep ipfs-daemon >/dev/null
if [ $? ]; then
  echo Unloading existing ipfs-daemon
  launchctl unload "$dest_dir/$plist"
fi

echo Loading ipfs-daemon
launchctl load "$dest_dir/$plist"
launchctl list | grep ipfs-daemon
