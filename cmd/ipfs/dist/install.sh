#!/bin/sh

bin=ipfs

# this script is currently brain dead.
# it merely tries two locations.
# in the future maybe use value of $PATH.

for binpath in /usr/local/bin /usr/bin; do
  if [ -d "$binpath" ]; then
    mv "$bin" "$binpath/$bin"
    echo "installed $binpath/$bin"
    exit 0
  fi
done
