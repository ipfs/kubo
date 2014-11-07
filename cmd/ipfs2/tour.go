package main

import (
	"bytes"
	"fmt"
	"io"
	"os"

	cmds "github.com/jbenet/go-ipfs/commands"
	config "github.com/jbenet/go-ipfs/config"
	internal "github.com/jbenet/go-ipfs/core/commands2/internal"
	tour "github.com/jbenet/go-ipfs/tour"
)

var cmdTour = &cmds.Command{

	Arguments: []cmds.Argument{
		cmds.Argument{"number", cmds.ArgString, false, false},
	},

	// TODO UsageLine: "tour [<number>]",
	// TODO Short:     "Take the IPFS Tour.",

	Help: `ipfs tour - Take the IPFS Tour.

    ipfs tour [<number>]   - Show tour topic. Default to current.
    ipfs tour next         - Show the next tour topic.
    ipfs tour list         - Show a list of topics.
    ipfs tour restart      - Restart the tour.

This is a tour that takes you through various IPFS concepts,
features, and tools to make sure you get up to speed with
IPFS very quickly. To start, run:

    ipfs tour
`,
	Subcommands: map[string]*cmds.Command{
		"list":    cmdIpfsTourList,
		"next":    cmdIpfsTourNext,
		"restart": cmdIpfsTourRestart,
	},
	Run: func(res cmds.Response, req cmds.Request) {

		out := new(bytes.Buffer)
		cfg := req.Context().Config
		strs, err := internal.ToStrings(req.Arguments())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		topic := tour.TopicID(cfg.Tour.Last)
		if len(strs) > 0 {
			topic = tour.TopicID(strs[0])
		}

		err = tourShow(out, topic)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(out)
	},
}

var cmdIpfsTourNext = &cmds.Command{
	Help: "Show the next IPFS Tour topic.",
	Run: func(res cmds.Response, req cmds.Request) {
		var w bytes.Buffer
		cfg := req.Context().Config
		path := req.Context().ConfigRoot

		topic := tour.NextTopic(tour.TopicID(cfg.Tour.Last))
		if err := tourShow(&w, topic); err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		// topic changed, not last. write it out.
		if string(topic) != cfg.Tour.Last {
			cfg.Tour.Last = string(topic)
			err := writeConfig(path, cfg)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}

		w.WriteTo(os.Stdout) // TODO write to res.SetValue
	},
}

var cmdIpfsTourRestart = &cmds.Command{
	Help: "Restart the IPFS Tour.",
	Run: func(res cmds.Response, req cmds.Request) {
		path := req.Context().ConfigRoot
		cfg := req.Context().Config

		cfg.Tour.Last = ""
		err := writeConfig(path, cfg)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
		}
	},
}

var cmdIpfsTourList = &cmds.Command{
	Help: "Show a list of IPFS Tour topics.",
	Run: func(res cmds.Response, req cmds.Request) {
		var w bytes.Buffer
		tourListCmd(&w, req.Context().Config)
		w.WriteTo(os.Stdout) // TODO use res.SetOutput(output)
	},
}

func tourListCmd(w io.Writer, cfg *config.Config) {

	lastid := tour.TopicID(cfg.Tour.Last)
	for _, id := range tour.IDs {
		c := ' '
		switch {
		case id == lastid:
			c = '*'
		case id.LessThan(lastid):
			c = '✓'
		}

		t := tour.Topics[id]
		fmt.Fprintf(w, "- %c %-5.5s %s\n", c, id, t.Title)
	}
}

func tourShow(w io.Writer, id tour.ID) error {
	t, found := tour.Topics[id]
	if !found {
		return fmt.Errorf("no topic with id: %s", id)
	}

	fmt.Fprintf(w, "Tour %s - %s\n\n%s\n", t.ID, t.Title, t.Text)
	return nil
}

// TODO share func
func writeConfig(path string, cfg *config.Config) error {
	filename, err := config.Filename(path)
	if err != nil {
		return err
	}
	return config.WriteConfigFile(filename, cfg)
}
