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
	Description: "An introduction to IPFS",
	Help: `This is a tour that takes you through various IPFS concepts,
features, and tools to make sure you get up to speed with
IPFS very quickly. To start, run:

    ipfs tour
`,

	Arguments: []cmds.Argument{
		cmds.StringArg("number", false, false, "The number of the topic you would like to tour"),
	},
	Subcommands: map[string]*cmds.Command{
		"list":    cmdIpfsTourList,
		"next":    cmdIpfsTourNext,
		"restart": cmdIpfsTourRestart,
	},
	Run: func(req cmds.Request) (interface{}, error) {

		out := new(bytes.Buffer)
		cfg, err := req.Context().GetConfig()
		if err != nil {
			return nil, err
		}

		strs, err := internal.CastToStrings(req.Arguments())
		if err != nil {
			return nil, err
		}

		topic := tour.TopicID(cfg.Tour.Last)
		if len(strs) > 0 {
			topic = tour.TopicID(strs[0])
		}

		err = tourShow(out, topic)
		if err != nil {
			return nil, err
		}

		return out, nil
	},
}

var cmdIpfsTourNext = &cmds.Command{
	Description: "Show the next IPFS Tour topic",

	Run: func(req cmds.Request) (interface{}, error) {
		var w bytes.Buffer
		path := req.Context().ConfigRoot
		cfg, err := req.Context().GetConfig()
		if err != nil {
			return nil, err
		}

		topic := tour.NextTopic(tour.TopicID(cfg.Tour.Last))
		if err := tourShow(&w, topic); err != nil {
			return nil, err
		}

		// topic changed, not last. write it out.
		if string(topic) != cfg.Tour.Last {
			cfg.Tour.Last = string(topic)
			err := writeConfig(path, cfg)
			if err != nil {
				return nil, err
			}
		}

		w.WriteTo(os.Stdout) // TODO write to res.SetValue
		return nil, nil
	},
}

var cmdIpfsTourRestart = &cmds.Command{
	Description: "Restart the IPFS Tour",

	Run: func(req cmds.Request) (interface{}, error) {
		path := req.Context().ConfigRoot
		cfg, err := req.Context().GetConfig()
		if err != nil {
			return nil, err
		}

		cfg.Tour.Last = ""
		err = writeConfig(path, cfg)
		if err != nil {
			return nil, err
		}
		return nil, nil
	},
}

var cmdIpfsTourList = &cmds.Command{
	Description: "Show a list of IPFS Tour topics",

	Run: func(req cmds.Request) (interface{}, error) {
		cfg, err := req.Context().GetConfig()
		if err != nil {
			return nil, err
		}

		var w bytes.Buffer
		tourListCmd(&w, cfg)
		w.WriteTo(os.Stdout) // TODO use res.SetOutput(output)
		return nil, nil
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
