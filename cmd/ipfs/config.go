package main

import (
	"errors"
	config "github.com/jbenet/go-ipfs/config"
	u "github.com/jbenet/go-ipfs/util"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/exec"
)

var cmdIpfsConfig = &cobra.Command{
	Use:   "config",
	Short: "Get/Set ipfs config values",
	Long: `ipfs config [<key>] [<value>] - Get/Set ipfs config values.

    ipfs config <key>          - Get value of <key>
    ipfs config <key> <value>  - Set value of <key> to <value>
    ipfs config --show         - Show config file
    ipfs config --edit         - Edit config file in $EDITOR

Examples:

  Get the value of the 'datastore.path' key:

      ipfs config datastore.path

  Set the value of the 'datastore.path' key:

      ipfs config datastore.path ~/.go-ipfs/datastore

`,
	Run: configCmd,
}

var (
	edit bool
	show bool
)

func init() {
	cmdIpfsConfig.Flags().BoolVarP(&edit, "edit", "e", false, "Edit config file in $EDITOR")
	cmdIpfsConfig.Flags().BoolVarP(&show, "show", "s", false, "Show config file")
	CmdIpfs.AddCommand(cmdIpfsConfig)
}

func configCmd(c *cobra.Command, inp []string) {

	// todo: implement --config filename flag.
	filename, err := config.Filename("")
	if err != nil {
		u.PErr(err.Error())
		return
	}

	// if editing, open the editor
	if edit {
		err = configEditor(filename)
		if err != nil {
			u.PErr(err.Error())
			return
		}
	}

	// if showing, cat the file
	if show {
		err = configCat(filename)
		if err != nil {
			u.PErr(err.Error())
			return
		}
	}

	if len(inp) == 0 {
		// "ipfs config" run without parameters
		u.POut(c.Long)
		return
	}

	// Getter (1 param)
	if len(inp) == 1 {
		value, err := config.ReadConfigKey(filename, inp[0])
		if err != nil {
			u.PErr("Failed to get config value: %s", err)
			return
		}

		strval, ok := value.(string)
		if ok {
			u.POut("%s\n", strval)
			return
		}

		if err := config.Encode(os.Stdout, value); err != nil {
			u.PErr("Failed to encode config value: %s", err)
			return
		}
		u.POut("\n")
		return
	}

	// Setter (>1 params)
	err = config.WriteConfigKey(filename, inp[0], inp[1])
	if err != nil {
		u.PErr("Failed to set config value: %s", err)
		return
	}
}

func configCat(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err = io.Copy(os.Stdout, file); err != nil {
		return err
	}
	u.POut("\n")
	return nil
}

func configEditor(filename string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		return errors.New("ENV variable $EDITOR not set")
	}

	cmd := exec.Command("sh", "-c", editor+" "+filename)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
