package commands

import (
	"bytes"
	"io/ioutil"
	"strings"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	mbase "github.com/multiformats/go-multibase"
)

var MbaseCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Encode and decode files with multibase format",
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
		Tagline: "Encode file or stdin into multibase string",
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
		buf, err := ioutil.ReadAll(file)
		if err != nil {
			return err
		}
		encoded := encoder.Encode(buf)
		reader := strings.NewReader(encoded)
		return resp.Emit(reader)
	},
}

var mbaseDecodeCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Decode multibase string to stdout",
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("encoded_file", true, false, "encoded data to decode").EnableStdin(),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		if err := req.ParseBodyArgs(); err != nil {
			return err
		}
		files := req.Files.Entries()
		file, err := cmdenv.GetFileArg(files)
		if err != nil {
			return err
		}
		encoded_data, err := ioutil.ReadAll(file)
		_, data, err := mbase.Decode(string(encoded_data))
		if err != nil {
			return err
		}
		reader := bytes.NewReader(data)
		return resp.Emit(reader)
	},
}
