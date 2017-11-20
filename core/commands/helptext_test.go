package commands

import (
	"strings"
	"testing"

	cmds "gx/ipfs/QmTwKPLyeRKuDawuy6CAn1kRj1FVoqBEM8sviAUWN7NW9K/go-ipfs-cmds"
)

func checkHelptextRecursive(t *testing.T, name []string, c *cmds.Command) {
	if c.Helptext.Tagline == "" {
		t.Errorf("%s has no tagline!", strings.Join(name, " "))
	}

	if c.Helptext.LongDescription == "" {
		t.Errorf("%s has no long description!", strings.Join(name, " "))
	}

	if c.Helptext.ShortDescription == "" {
		t.Errorf("%s has no short description!", strings.Join(name, " "))
	}

	if c.Helptext.Synopsis == "" {
		t.Errorf("%s has no synopsis!", strings.Join(name, " "))
	}

	for subname, sub := range c.Subcommands {
		checkHelptextRecursive(t, append(name, subname), sub)
	}
}

func TestHelptexts(t *testing.T) {
	t.Skip("sill isn't 100%")
	Root.ProcessHelp()
	checkHelptextRecursive(t, []string{"ipfs"}, Root)
}
