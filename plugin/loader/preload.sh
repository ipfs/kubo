#!/bin/bash

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

to_preload() {
	awk 'NF' "$DIR/preload_list" | sed '/^#/d'
}

cat <<EOL
package loader

import (
	"github.com/ipfs/go-ipfs/plugin"
EOL

to_preload | while read -r name path num; do
	echo "plugin$name \"$path\""
done | sort -u

cat <<EOL
)

var preloadPlugins = []plugin.Plugin{
EOL

to_preload | while read -r name path num; do
	echo "plugin$name.Plugins[$num],"
done


echo "}"
