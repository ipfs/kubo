package main

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	"github.com/jbenet/go-ipfs/core/commands"
)

var cmdIpfsResolve = &commander.Command{
	UsageLine: "resolve",
	Short:     "resolve an ipns name to a <ref>",
	Long: `ipfs resolve [<name>] - Resolve an ipns name to a <ref>.

IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values. In resolve, the
default value of <name> is your own identity public key.


Examples:

Resolve the value of your identity:

  > ipfs name resolve
  QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Resolve te value of another name:

  > ipfs name resolve QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n
  QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

`,
	Run:  resolveCmd,
	Flag: *flag.NewFlagSet("ipfs-resolve", flag.ExitOnError),
}

var resolveCmd = makeCommand(command{
	name:   "resolve",
	args:   0,
	flags:  nil,
	online: true,
	cmdFn:  commands.Resolve,
})
