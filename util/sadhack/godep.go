package util

// FIXME: we need the go-random/random utility for our sharness test wich depends on go-humanize
// Godep will drop it if we dont use it in ipfs. There should be a better way to do this.
import _ "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/dustin/go-humanize"

// similar to the above, only used in the tests makefile
import _ "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/whyrusleeping/iptb"

import _ "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/chriscool/go-sleep"

import _ "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/hashicorp/yamux"
