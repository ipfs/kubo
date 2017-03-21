package commands

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	cmds "github.com/ipfs/go-ipfs/commands"

	ci "gx/ipfs/QmPGxZ1DP2w45WcogpW1h43BvseXbfke9N91qotpoQcUeS/go-libp2p-crypto"
	peer "gx/ipfs/QmWUswjn261LSyVxWAEpMVtPdy8zmKBJJfBpG3Qdpa8ZsE/go-libp2p-peer"
)

var KeyCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create and list IPNS name keypairs",
		ShortDescription: `
'ipfs key gen' generates a new keypair for usage with IPNS and 'ipfs name publish'.

  > ipfs key gen --type=rsa --size=2048 mykey
  > ipfs name publish --key=mykey QmSomeHash

'ipfs key list' lists the available keys.

  > ipfs key list
  self
  mykey
		`,
	},
	Subcommands: map[string]*cmds.Command{
		"gen":  KeyGenCmd,
		"list": KeyListCmd,
	},
}

type KeyOutput struct {
	Name string
	Id   string
}

type KeyOutputList struct {
	Keys []KeyOutput
}

var KeyGenCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Create a new keypair",
	},
	Options: []cmds.Option{
		cmds.StringOption("type", "t", "type of the key to create [rsa, ed25519]"),
		cmds.IntOption("size", "s", "size of the key to generate"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("name", true, false, "name of key to create"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		typ, f, err := req.Option("type").String()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !f {
			res.SetError(fmt.Errorf("please specify a key type with --type"), cmds.ErrNormal)
			return
		}

		size, sizefound, err := req.Option("size").Int()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		name := req.Arguments()[0]
		if name == "self" {
			res.SetError(fmt.Errorf("cannot create key with name 'self'"), cmds.ErrNormal)
			return
		}

		var sk ci.PrivKey
		var pk ci.PubKey

		switch typ {
		case "rsa":
			if !sizefound {
				res.SetError(fmt.Errorf("please specify a key size with --size"), cmds.ErrNormal)
				return
			}

			priv, pub, err := ci.GenerateKeyPairWithReader(ci.RSA, size, rand.Reader)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			sk = priv
			pk = pub
		case "ed25519":
			priv, pub, err := ci.GenerateEd25519Key(rand.Reader)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			sk = priv
			pk = pub
		default:
			res.SetError(fmt.Errorf("unrecognized key type: %s", typ), cmds.ErrNormal)
			return
		}

		err = n.Repo.Keystore().Put(name, sk)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		pid, err := peer.IDFromPublicKey(pk)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&KeyOutput{
			Name: name,
			Id:   pid.Pretty(),
		})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			k, ok := res.Output().(*KeyOutput)
			if !ok {
				return nil, fmt.Errorf("expected a KeyOutput as command result")
			}

			return strings.NewReader(k.Id + "\n"), nil
		},
	},
	Type: KeyOutput{},
}

var KeyListCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List all local keypairs",
	},
	Options: []cmds.Option{
		cmds.BoolOption("l", "Show extra information about keys."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		keys, err := n.Repo.Keystore().List()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		sort.Strings(keys)

		list := make([]KeyOutput, 0, len(keys)+1)

		list = append(list, KeyOutput{Name: "self", Id: n.Identity.Pretty()})

		for _, key := range keys {
			privKey, err := n.Repo.Keystore().Get(key)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			pubKey := privKey.GetPublic()

			pid, err := peer.IDFromPublicKey(pubKey)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}

			list = append(list, KeyOutput{Name: key, Id: pid.Pretty()})
		}

		res.SetOutput(&KeyOutputList{list})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: keyOutputListMarshaler,
	},
	Type: KeyOutputList{},
}

func keyOutputListMarshaler(res cmds.Response) (io.Reader, error) {
	withId, _, _ := res.Request().Option("l").Bool()

	list, ok := res.Output().(*KeyOutputList)
	if !ok {
		return nil, errors.New("failed to cast []KeyOutput")
	}

	buf := new(bytes.Buffer)
	w := tabwriter.NewWriter(buf, 1, 2, 1, ' ', 0)
	for _, s := range list.Keys {
		if withId {
			fmt.Fprintf(w, "%s\t%s\t\n", s.Id, s.Name)
		} else {
			fmt.Fprintf(w, "%s\n", s.Name)
		}
	}
	w.Flush()
	return buf, nil
}
