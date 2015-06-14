package commands

import (
	cmds "github.com/ipfs/go-ipfs/commands"
	crypt "github.com/ipfs/go-ipfs/crypt"
)

var CryptCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "perform ipfs cryptographic operations",
		ShortDescription: ``,
	},
	Subcommands: map[string]*cmds.Command{
		"en": encryptCmd,
		"de": decryptCmd,
	},
}

var encryptCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "encrypt data using ipfs keypairs",
		ShortDescription: ``,
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("data", true, false, "data to encrypt").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("k", "key", "key to use for encryption (default 'local')"),
		cmds.StringOption("o", "output", "output file name (default stdout)"),
		cmds.StringOption("p", "cipher", "cipher to use for encryption"),
		cmds.StringOption("m", "mode", "block cipher mode to use for encryption"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		k, found, err := req.Option("k").String()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !found {
			k = "local"
		}

		key, err := nd.Repo.Keystore().GetKey(k)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fi, err := req.Files().NextFile()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		stream, err := crypt.EncryptStreamWithKey(fi, key.GetPublic())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(stream)
	},
}

var decryptCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "decrypt data using ipfs keypairs",
		ShortDescription: ``,
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("data", true, false, "data to decrypt").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption("k", "key", "key to use for encryption (default 'local')"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		k, found, err := req.Option("k").String()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !found {
			k = "local"
		}

		key, err := nd.Repo.Keystore().GetKey(k)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		fi, err := req.Files().NextFile()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		stream, err := crypt.DecryptStreamWithKey(fi, key)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(stream)
	},
}
