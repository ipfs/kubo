package main

import (
  "github.com/gonuts/flag"
  "github.com/jbenet/commander"
  u "github.com/jbenet/go-ipfs/util"
  "strings"
  "time"
)

var CmdIpfs = &commander.Command{
  UsageLine: "ipfs [<flags>] <command> [<args>]",
  Short:     "global versioned p2p merkledag file system",
  Long: `ipfs - global versioned p2p merkledag file system

Basic commands:

    add <path>    Add an object to ipfs.
    cat <ref>     Show ipfs object data.
    ls <ref>      List links from an object.
    refs <ref>    List link hashes from an object.

Tool commands:

    config      Manage configuration.
    version     Show ipfs version information.
    commands    List all available commands.

Advanced Commands:

    mount       Mount a ipfs a read-only mountpoint.

Use "ipfs help <command>" for more information about a command.
`,
  Run: ipfsCmd,
  Subcommands: []*commander.Command{
    cmdIpfsVersion,
    // cmdIpfsConfig,
    cmdIpfsCommands,
  },
  Flag: *flag.NewFlagSet("ipfs", flag.ExitOnError),
}

func ipfsCmd(c *commander.Command, args []string) error {
  u.POut(c.Long)
  return nil
}

var cmdIpfsCommands = &commander.Command{
  UsageLine: "commands",
  Short:     "List all available commands.",
  Long: `ipfs commands - List all available commands.

    Lists all available commands (and sub-commands) and exits.
  `,
  Run: commandsCmd,
  Subcommands: []*commander.Command{
    cmdIpfsCommandsHelp,
  },
}

func commandsCmd(c *commander.Command, args []string) error {
  var listCmds func(c *commander.Command)
  listCmds = func(c *commander.Command) {
    u.POut("%s\n", c.FullSpacedName())
    for _, sc := range c.Subcommands {
      listCmds(sc)
    }
  }

  listCmds(c.Parent)
  return nil
}

var cmdIpfsCommandsHelp = &commander.Command{
  UsageLine: "help",
  Short:     "List all available commands' help pages.",
  Long: `ipfs commands help - List all available commands's help pages.

    Shows the pages of all available commands (and sub-commands) and exits.
    Outputs a markdown document.
  `,
  Run: commandsHelpCmd,
}

func commandsHelpCmd(c *commander.Command, args []string) error {
  u.POut(referenceHeaderMsg)
  u.POut("Generated on %s.\n\n", time.Now().UTC().Format("2006-01-02"))

  var printCmds func(*commander.Command, int)
  printCmds = func(c *commander.Command, level int) {
    u.POut("%s ", strings.Repeat("#", level))
    u.POut("%s\n\n", c.FullSpacedName())
    u.POut("```\n")
    u.POut("%s\n", c.Long)
    u.POut("```\n\n")

    for _, sc := range c.Subcommands {
      printCmds(sc, level+1)
    }
  }

  printCmds(c.Parent.Parent, 1)
  return nil
}

const referenceHeaderMsg = `
# ipfs command reference

This document lists every ipfs command (including subcommands), along with
its help page. It can be viewed by running 'ipfs commands help'.

`
