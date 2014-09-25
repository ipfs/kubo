package main

import (
	"errors"
	"fmt"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gonuts/flag"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
	config "github.com/jbenet/go-ipfs/config"
	u "github.com/jbenet/go-ipfs/util"
	"io"
	"os"
	"os/exec"
)

var cmdIpfsConfig = &commander.Command{
	UsageLine: "config",
	Short:     "Get/Set ipfs config values",
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
	Run:  configCmd,
	Flag: *flag.NewFlagSet("ipfs-config", flag.ExitOnError),
}

func init() {
	cmdIpfsConfig.Flag.Bool("edit", false, "Edit config file in $EDITOR")
	cmdIpfsConfig.Flag.Bool("show", false, "Show config file")
}

func configCmd(c *commander.Command, inp []string) error {

	confdir, err := getConfigDir(c.Parent)
	if err != nil {
		return err
	}

	filename, err := config.GetConfigFilePath(confdir)
	if err != nil {
		return err
	}

	// if editing, open the editor
	if c.Flag.Lookup("edit").Value.Get().(bool) {
		return configEditor(filename)
	}

	// if showing, cat the file
	if c.Flag.Lookup("show").Value.Get().(bool) {
		return configCat(filename)
	}

	if len(inp) == 0 {
		// "ipfs config" run without parameters
		u.POut(c.Long)
		return nil
	}

	// Getter (1 param)
	if len(inp) == 1 {
		value, err := config.ReadConfigKey(filename, inp[0])
		if err != nil {
			return fmt.Errorf("Failed to get config value: %s", err)
		}

		strval, ok := value.(string)
		if ok {
			u.POut("%s\n", strval)
			return nil
		}

		if err := config.Encode(os.Stdout, value); err != nil {
			return fmt.Errorf("Failed to encode config value: %s", err)
		}
		u.POut("\n")
		return nil
	}

	// Setter (>1 params)
	err = config.WriteConfigKey(filename, inp[0], inp[1])
	if err != nil {
		return fmt.Errorf("Failed to set config value: %s", err)
	}

	return nil
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
