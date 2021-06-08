package commands

import (
	"io/ioutil"
	"os"

	cmds "github.com/ipfs/go-ipfs-cmds"
	mbase "github.com/multiformats/go-multibase"
)

var MbaseCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "",
	},
	Subcommands: map[string]*cmds.Command{
		"encode": mbaseEncodeCmd,
		"decode": mbaseDecodeCmd,
	},
	Extra: CreateCmdExtras(SetDoesNotUseRepo(true)),
}

const (
	mbaseOptionName = "b"
)

var mbaseEncodeCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:         "",
		LongDescription: "",
	},
	Options: []cmds.Option{
		cmds.StringOption(mbaseOptionName, "multibase encoding").WithDefault("identity"),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		encoderName, _ := req.Options[mbaseOptionName].(string)
		encoder, err := mbase.EncoderByName(encoderName)
		if err != nil {
			return err
		}
		b, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
		return resp.Emit(encoder.Encode(b))
	},
}

var mbaseDecodeCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:         "",
		LongDescription: "",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("data", true, false, "encoded data to decode").EnableStdin(),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		if err := req.ParseBodyArgs(); err != nil {
			return err
		}
		data := req.Arguments[0]
		_, buf, err := mbase.Decode(data)
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(buf)
		return err
	},
}
