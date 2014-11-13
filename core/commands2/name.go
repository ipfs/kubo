package commands

import cmds "github.com/jbenet/go-ipfs/commands"

type IpnsEntry struct {
	Name  string
	Value string
}

var nameCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "IPFS namespace (IPNS) tool",
		ShortDescription: `
IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values. In both publish
and resolve, the default value of <name> is your own identity public key.
`,
		LongDescription: `
IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values. In both publish
and resolve, the default value of <name> is your own identity public key.


Examples:

Publish a <ref> to your identity name:

  > ipfs name publish QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  published name QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n to QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Publish a <ref> to another public key:

  > ipfs name publish QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy
  published name QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n to QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Resolve the value of your identity:

  > ipfs name resolve
  QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Resolve the value of another name:

  > ipfs name resolve QmbCMUZw6JFeZ7Wp9jkzbye3Fzp2GGcPgC3nmeUjfVF87n
  QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

`,
	},

	Subcommands: map[string]*cmds.Command{
		"publish": publishCmd,
		"resolve": resolveCmd,
	},
}
