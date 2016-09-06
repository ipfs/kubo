#!/bin/sh

bin=ipfs
binpaths="/usr/local/bin /usr/bin"

# this script is currently brain dead.
# it merely tries two locations.
# in the future maybe use value of $PATH.

for binpath in $binpaths; do
  if [ -d "$binpath" ]; then
    mv "$bin" "$binpath/$bin"
    echo "installed $binpath/$bin"
    exit 0
  fi
done

echo "cannot install $bin in one of the directories $binpaths"
exit 1
