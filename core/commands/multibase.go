package commands

import (
	"bytes"
	"strings"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
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
	Arguments: []cmds.Argument{
		cmds.FileArg("file", true, false, "data to encode").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption(mbaseOptionName, "multibase encoding").WithDefault("identity"),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		if err := req.ParseBodyArgs(); err != nil {
			return err
		}
		encoderName, _ := req.Options[mbaseOptionName].(string)
		encoder, err := mbase.EncoderByName(encoderName)
		if err != nil {
			return err
		}
		files := req.Files.Entries()
		file, err := cmdenv.GetFileArg(files)
		if err != nil {
			return err
		}
		size, err := file.Size()
		if err != nil {
			return err
		}
		buf := make([]byte, size)
		n, err := file.Read(buf)
		if err != nil {
			return err
		}
		encoded := encoder.Encode(buf[:n])
		reader := strings.NewReader(encoded)
		return resp.Emit(reader)
	},
}

var mbaseDecodeCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:         "",
		LongDescription: "",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("encoded_data", true, false, "encoded data to decode").EnableStdin(),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		if err := req.ParseBodyArgs(); err != nil {
			return err
		}
		encoded_data := req.Arguments[0]
		_, data, err := mbase.Decode(encoded_data)
		if err != nil {
			return err
		}
		reader := bytes.NewReader(data)
		return resp.Emit(reader)
	},
}
