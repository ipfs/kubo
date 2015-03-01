package util

// FIXME: we need the go-random/random utility for our sharness test wich depends on go-humanize
// Godep will drop it if we dont use it in ipfs. There should be a better way to do this.
import _ "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/dustin/go-humanize"
