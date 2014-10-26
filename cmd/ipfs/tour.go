// +build linux darwin freebsd

package main

import (
	"fmt"

	config "github.com/jbenet/go-ipfs/config"
	tour "github.com/jbenet/go-ipfs/tour"

	commander "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/commander"
)

var cmdIpfsTour = &commander.Command{
	UsageLine: "tour [<number>]",
	Short:     "Take the IPFS Tour.",
	Long: `ipfs tour - Take the IPFS Tour.

    ipfs tour [<number>]   - Show tour topic. Default to current.
    ipfs tour next         - Show the next tour topic.
    ipfs tour list         - Show a list of topics.
    ipfs tour restart      - Restart the tour.

This is a tour that takes you through various IPFS concepts,
features, and tools to make sure you get up to speed with
IPFS very quickly. To start, run:

    ipfs tour
`,
	Run: tourCmd,
	Subcommands: []*commander.Command{
		cmdIpfsTourNext,
		cmdIpfsTourList,
		cmdIpfsTourRestart,
	},
}

var cmdIpfsTourNext = &commander.Command{
	UsageLine: "next",
	Short:     "Show the next IPFS Tour topic.",
	Run:       tourNextCmd,
}

var cmdIpfsTourList = &commander.Command{
	UsageLine: "list",
	Short:     "Show a list of IPFS Tour topics.",
	Run:       tourListCmd,
}

var cmdIpfsTourRestart = &commander.Command{
	UsageLine: "restart",
	Short:     "Restart the IPFS Tour.",
	Run:       tourRestartCmd,
}

func tourCmd(c *commander.Command, inp []string) error {
	cfg, err := getConfig(c)
	if err != nil {
		return err
	}

	topic := tour.TopicID(cfg.Tour.Last)
	if len(inp) > 0 {
		topic = tour.TopicID(inp[0])
	}
	return tourShow(topic)
}

func tourNextCmd(c *commander.Command, _ []string) error {
	cfg, err := getConfig(c)
	if err != nil {
		return err
	}

	topic := tour.NextTopic(tour.TopicID(cfg.Tour.Last))
	if err := tourShow(topic); err != nil {
		return err
	}

	// if topic didn't change (last) done
	if string(topic) == cfg.Tour.Last {
		return nil
	}

	// topic changed, not last. write it out.
	cfg.Tour.Last = string(topic)
	return writeConfig(c, cfg)
}

func tourListCmd(c *commander.Command, _ []string) error {
	cfg, err := getConfig(c)
	if err != nil {
		return err
	}
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
		fmt.Printf("- %c %5.5s %s\n", c, id, t.Title)
	}
	return nil
}

func tourRestartCmd(c *commander.Command, _ []string) error {
	cfg, err := getConfig(c)
	if err != nil {
		return err
	}

	cfg.Tour.Last = ""
	return writeConfig(c, cfg)
}

func tourShow(id tour.ID) error {
	t, found := tour.Topics[id]
	if !found {
		return fmt.Errorf("no topic with id: %s", id)
	}

	fmt.Printf("Tour %s - %s\n\n%s\n", t.ID, t.Title, t.Text)
	return nil
}

func lastTour(cfg *config.Config) string {
	return ""
}
