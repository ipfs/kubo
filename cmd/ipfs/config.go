package main

import (
	"github.com/jbenet/commander"
	config "github.com/jbenet/go-ipfs/config"
	u "github.com/jbenet/go-ipfs/util"
	"os"
	"os/exec"
)

var cmdIpfsConfig = &commander.Command{
	UsageLine: "config",
	Short:     "See and Edit ipfs options",
	Long: `ipfs config - See or Edit ipfs configuration.

    See specific config's values with:
    	ipfs config datastore.path
    Assign a new value with:
    	ipfs config datastore.path ~/.go-ipfs/datastore

    Open the config file in your editor(from $EDITOR):
    	ipfs config edit
  `,
	Run: configCmd,
	Subcommands: []*commander.Command{
		cmdIpfsConfigEdit,
	},
}

var cmdIpfsConfigEdit = &commander.Command{
	UsageLine: "edit",
	Short:     "Opens the configuration file in the editor.",
	Long: `Looks up environment variable $EDITOR and 
	attempts to open the config file with it.
  `,
	Run: configEditCmd,
}

func configCmd(c *commander.Command, inp []string) error {
	if len(inp) == 0 {
		// "ipfs config" run without parameters
		u.POut(c.Long + "\n")
		return nil
	}

	if len(inp) == 1 {
		// "ipfs config" run without one parameter, so this is a value getter
		value, err := config.GetValueInConfigFile(inp[0])
		if err != nil {
			u.POut("Failed to get config value: " + err.Error() + "\n")
		} else {
			u.POut(value + "\n")
		}
		return nil
	}

	// "ipfs config" run without two parameter, so this is a value setter
	err := config.SetValueInConfigFile(inp[0], inp[1:])
	if err != nil {
		u.POut("Failed to set config value: " + err.Error() + "\n")
	}
	return nil
}

func configEditCmd(c *commander.Command, _ []string) error {
	if editor := os.Getenv("EDITOR"); editor == "" {
		u.POut("ENVIRON variable $EDITOR is not assigned \n")
	} else {
		exec.Command("sh", "-c", editor+" "+config.DefaultConfigFilePath).Start()
	}
	return nil
}
