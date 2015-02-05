package tour

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"

	cmds "github.com/jbenet/go-ipfs/thirdparty/commands"
	config "github.com/jbenet/go-ipfs/repo/config"
	fsrepo "github.com/jbenet/go-ipfs/repo/fsrepo"
	tour "github.com/jbenet/go-ipfs/tour"
)

var TourCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "An introduction to IPFS",
		ShortDescription: `
This is a tour that takes you through various IPFS concepts,
features, and tools to make sure you get up to speed with
IPFS very quickly. To start, run:

    ipfs tour
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("id", false, false, "The id of the topic you would like to tour"),
	},
	Subcommands: map[string]*cmds.Command{
		"list":    cmdIpfsTourList,
		"next":    cmdIpfsTourNext,
		"restart": cmdIpfsTourRestart,
	},
	Run: tourRunFunc,
}

func tourRunFunc(req cmds.Request, res cmds.Response) {

	cfg, err := req.Context().GetConfig()
	if err != nil {
		res.SetError(err, cmds.ErrNormal)
		return
	}

	id := tour.TopicID(cfg.Tour.Last)
	if len(req.Arguments()) > 0 {
		id = tour.TopicID(req.Arguments()[0])
	}

	var w bytes.Buffer
	defer w.WriteTo(os.Stdout)
	t, err := tourGet(id)
	if err != nil {

		// If no topic exists for this id, we handle this error right here.
		// To help the user achieve the task, we construct a response
		// comprised of...
		// 1) a simple error message
		// 2) the full list of topics

		fmt.Fprintln(&w, "ERROR")
		fmt.Fprintln(&w, err)
		fmt.Fprintln(&w, "")
		fprintTourList(&w, tour.TopicID(cfg.Tour.Last))

		return
	}

	fprintTourShow(&w, t)
}

var cmdIpfsTourNext = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show the next IPFS Tour topic",
	},

	Run: func(req cmds.Request, res cmds.Response) {
		var w bytes.Buffer
		path := req.Context().ConfigRoot
		cfg, err := req.Context().GetConfig()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		id := tour.NextTopic(tour.TopicID(cfg.Tour.Last))
		topic, err := tourGet(id)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
		if err := fprintTourShow(&w, topic); err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		// topic changed, not last. write it out.
		if string(id) != cfg.Tour.Last {
			cfg.Tour.Last = string(id)
			err := writeConfig(path, cfg)
			if err != nil {
				res.SetError(err, cmds.ErrNormal)
				return
			}
		}

		w.WriteTo(os.Stdout)
	},
}

var cmdIpfsTourRestart = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Restart the IPFS Tour",
	},

	Run: func(req cmds.Request, res cmds.Response) {
		path := req.Context().ConfigRoot
		cfg, err := req.Context().GetConfig()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		cfg.Tour.Last = ""
		err = writeConfig(path, cfg)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

var cmdIpfsTourList = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show a list of IPFS Tour topics",
	},

	Run: func(req cmds.Request, res cmds.Response) {
		cfg, err := req.Context().GetConfig()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		var w bytes.Buffer
		fprintTourList(&w, tour.TopicID(cfg.Tour.Last))
		w.WriteTo(os.Stdout)
	},
}

func fprintTourList(w io.Writer, lastid tour.ID) {
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

// fprintTourShow writes a text-formatted topic to the writer
func fprintTourShow(w io.Writer, t *tour.Topic) error {
	tmpl := `
Tour {{ .ID }} - {{ .Title }}

{{ .Text }}

`
	ttempl, err := template.New("tour").Parse(tmpl)
	if err != nil {
		return err
	}
	return ttempl.Execute(w, t)
}

// tourGet returns the topic given its ID. Returns an error if topic does not
// exist.
func tourGet(id tour.ID) (*tour.Topic, error) {
	t, found := tour.Topics[id]
	if !found {
		return nil, fmt.Errorf("no topic with id: %s", id)
	}
	return &t, nil
}

// TODO share func
func writeConfig(path string, cfg *config.Config) error {
	// NB: This needs to run on the daemon.
	r := fsrepo.At(path)
	if err := r.Open(); err != nil {
		return err
	}
	defer r.Close()
	return r.SetConfig(cfg)
}
