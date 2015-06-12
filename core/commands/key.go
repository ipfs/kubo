package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	cmds "github.com/ipfs/go-ipfs/commands"
	ci "github.com/ipfs/go-ipfs/p2p/crypto"
	peer "github.com/ipfs/go-ipfs/p2p/peer"
)

const PeerKeyName = "local"

var KeyCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with IPFS keypairs",
		ShortDescription: `
'ipfs key' is a command used to manage ipfs keypairs.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"gen": keyGenCmd,
	},
}

type KeyOutput struct {
	Keyname string
	KeyID   string
	Bits    int
	Type    string
}

var keyGenCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Generate a new named ipfs keypair",
		ShortDescription: `
'ipfs key gen' is a command used to generate new keypairs.
If any options are not given, the command will go into interactive mode and prompt
the user for the missing fields.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("keyname", true, false, "The name of the key to create").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.IntOption("b", "bits", "bitsize of key to generate (default = 2048)"),
		cmds.StringOption("t", "type", "type of key to generate (default = RSA)"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		kname := req.Arguments()[0]
		if kname == PeerKeyName {
			res.SetError(errors.New("cannot name key 'local', would overwrite peer key"), cmds.ErrNormal)
			return
		}

		_, err = nd.Repo.Keystore().GetKey(kname)
		if err != nil && !os.IsNotExist(err) {
			res.SetError(fmt.Errorf("error checking for key in keystore: %s", err), cmds.ErrNormal)
			return
		}
		if err == nil {
			res.SetError(errors.New("key with that name already exists"), cmds.ErrNormal)
			return
		}

		bits, found, err := req.Option("b").Int()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if !found {
			bits = 2048
		}

		typ, found, err := req.Option("t").String()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if !found {
			typ = "RSA"
		}

		var ntyp int
		switch typ {
		case "RSA":
			ntyp = ci.RSA
		default:
			res.SetError(errors.New("unrecognized key type"), cmds.ErrNormal)
			return
		}

		sk, pk, err := ci.GenerateKeyPair(ntyp, bits)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		pid, err := peer.IDFromPublicKey(pk)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = nd.Repo.Keystore().PutKey(kname, sk)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&KeyOutput{
			Type:    typ,
			KeyID:   pid.Pretty(),
			Keyname: kname,
			Bits:    bits,
		})
	},
	Type: KeyOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			ko := res.Output().(*KeyOutput)
			buf := new(bytes.Buffer)
			fmt.Fprintf(buf, "created new %s key: %s\n", ko.Type, ko.KeyID)
			return buf, nil
		},
	},
}
