package commands

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	cmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core/corenet"
	crypt "github.com/ipfs/go-ipfs/crypt"
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
		"gen":  keyGenCmd,
		"send": keySendCmd,
		"recv": keyRecvCmd,
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
		cmds.StringArg("keyname", true, false, "The name of the key to create"),
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

var keySendCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "send an ipfs key to a known peer",
		ShortDescription: `
'ipfs key send' is a command used to share keypairs with other trusted users.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("keyname", true, false, "The name of the key to send"),
		cmds.StringArg("peer", true, false, "The peer ID of the recipient"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !nd.OnlineMode() {
			res.SetError(errors.New("this command must be run with a live daemon"), cmds.ErrNormal)
			return
		}

		kname := req.Arguments()[0]
		target, err := peer.IDB58Decode(req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if kname == "local" {
			res.SetError(errors.New("you cannot send your peers ID key"), cmds.ErrNormal)
		}

		tosend, err := nd.Repo.Keystore().GetKey(kname)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		privkb, err := ci.MarshalPrivateKey(tosend)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		list, err := corenet.Listen(nd, "/ipfs/keys/"+target.Pretty())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		defer list.Close()

		con, err := list.Accept()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		defer con.Close()

		if con.Conn().RemotePeer() != target {
			res.SetError(errors.New("different peer than expected tried to connect!"), cmds.ErrNormal)
			return
		}

		// get targets public key to encrypt the key we will send
		pubk := nd.Peerstore.PubKey(target)
		br := bytes.NewReader(privkb)
		encread, err := crypt.EncryptStreamWithKey(br, pubk)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		_, err = io.Copy(con, encread)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		kid, err := peer.IDFromPrivateKey(tosend)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&KeyOutput{
			KeyID: kid.Pretty(),
		})
	},
	Type: KeyOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			ko := res.Output().(*KeyOutput)
			buf := new(bytes.Buffer)
			fmt.Fprintf(buf, "sent key: %s\n", ko.KeyID)
			return buf, nil
		},
	},
}

var keyRecvCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "receive an ipfs key from a known peer",
		ShortDescription: `
'ipfs key recv' is a command used to receive a keypair from another trusted user
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("keyname", true, false, "The name for the received key"),
		cmds.StringArg("peer", true, false, "The peer ID of the sender"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		nd, err := req.Context().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !nd.OnlineMode() {
			res.SetError(errors.New("this command must be run with a live daemon"), cmds.ErrNormal)
			return
		}

		kname := req.Arguments()[0]
		sender, err := peer.IDB58Decode(req.Arguments()[1])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if kname == "local" {
			res.SetError(errors.New("you cannot send your peers ID key"), cmds.ErrNormal)
		}

		con, err := corenet.Dial(nd, sender, "/ipfs/keys/"+nd.Identity.Pretty())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		read, err := crypt.DecryptStreamWithKey(con, nd.PrivateKey)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		keybytes, err := ioutil.ReadAll(read)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		newkey, err := ci.UnmarshalPrivateKey(keybytes)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		err = nd.Repo.Keystore().PutKey(kname, newkey)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		kid, err := peer.IDFromPrivateKey(newkey)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(&KeyOutput{
			KeyID: kid.Pretty(),
		})
	},
	Type: KeyOutput{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			ko := res.Output().(*KeyOutput)
			buf := new(bytes.Buffer)
			fmt.Fprintf(buf, "received new key: %s\n", ko.KeyID)
			return buf, nil
		},
	},
}
