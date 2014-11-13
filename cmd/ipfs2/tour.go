package main

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"os"

	cmds "github.com/jbenet/go-ipfs/commands"
	config "github.com/jbenet/go-ipfs/config"
	internal "github.com/jbenet/go-ipfs/core/commands2/internal"
	tour "github.com/jbenet/go-ipfs/tour"
	"github.com/jbenet/go-ipfs/util"
)

// TODO the parent function now uses tourOutput. Migrate the children to also
// use the tourOutput struct

var tourCmd = &cmds.Command{
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
		cmds.StringArg("number", false, false, "The number of the topic you would like to tour"),
	},
	Subcommands: map[string]*cmds.Command{
		"list":    cmdIpfsTourList,
		"next":    cmdIpfsTourNext,
		"restart": cmdIpfsTourRestart,
	},
	Run: tourRunFunc,
	Marshalers: cmds.MarshalerMap{
		cmds.Text: tourTextMarshaler,
	},
	Type: &tourOutput{},
}

// tourOutput is a union type. It either contains a Topic or it contains the
// list of Topics and an Error.
type tourOutput struct {
	Topic *tour.Topic

	Topics []tour.Topic
	Error  error
}

func tourTextMarshaler(r cmds.Response) ([]byte, error) {
	output, ok := r.Output().(*tourOutput)
	if !ok {
		return nil, util.ErrCast()
	}
	// can be listing when error
	var buf bytes.Buffer
	err := printTourOutput(&buf, output)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func printTourOutput(w io.Writer, output *tourOutput) error {
	tmpl := `{{ if .Error }}
ERROR
	{{ .Error }}
TOPICS
	{{ range $topic := .Topics }}
	{{ $topic.ID }} - {{ $topic.Title }} {{ end }}
{{ else if .Topic }}
Tour {{ .Topic.ID }} - {{ .Topic.Title }}

{{ .Topic.Text }}
{{ end }}
`
	tourTmpl, err := template.New("tour").Parse(tmpl)
	if err != nil {
		return err
	}
	return tourTmpl.Execute(w, output)
}

func tourRunFunc(req cmds.Request) (interface{}, error) {

	cfg, err := req.Context().GetConfig()
	if err != nil {
		return nil, err
	}

	strs, err := internal.CastToStrings(req.Arguments())
	if err != nil {
		return nil, err
	}

	id := tour.TopicID(cfg.Tour.Last)
	if len(strs) > 0 {
		id = tour.TopicID(strs[0])
	}

	t, err := tourGet(id)
	if err != nil {

		// If no topic exists for this id, we handle this error right here.
		// To help the user achieve the task, we construct a response
		// comprised of...
		// 1) a simple error message
		// 2) the full list of topics

		output := &tourOutput{
			Error: err,
		}
		for _, id := range tour.IDs {
			t, ok := tour.Topics[id]
			if !ok {
				return nil, err
			}
			output.Topics = append(output.Topics, t)
		}

		return output, nil
		// return nil, cmds.ClientError(err.Error())
	}

	return &tourOutput{Topic: t}, nil
}

// TODO use tourOutput like parent command
var cmdIpfsTourNext = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show the next IPFS Tour topic",
	},

	Run: func(req cmds.Request) (interface{}, error) {
		var w bytes.Buffer
		path := req.Context().ConfigRoot
		cfg, err := req.Context().GetConfig()
		if err != nil {
			return nil, err
		}

		id := tour.NextTopic(tour.TopicID(cfg.Tour.Last))
		topic, err := tourGet(id)
		if err != nil {
			return nil, err
		}
		if err := tourShow(&w, topic); err != nil {
			return nil, err
		}

		// topic changed, not last. write it out.
		if string(id) != cfg.Tour.Last {
			cfg.Tour.Last = string(id)
			err := writeConfig(path, cfg)
			if err != nil {
				return nil, err
			}
		}

		w.WriteTo(os.Stdout) // TODO write to res.SetValue
		return w, nil
	},
}

var cmdIpfsTourRestart = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Restart the IPFS Tour",
	},

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

// TODO use tourOutput like parent command
var cmdIpfsTourList = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show a list of IPFS Tour topics",
	},

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

func tourShow(w io.Writer, t *tour.Topic) error {
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

// tourGet returns an error if topic does not exist
func tourGet(id tour.ID) (*tour.Topic, error) {
	t, found := tour.Topics[id]
	if !found {
		return nil, fmt.Errorf("no topic with id: %s", id)
	}
	return &t, nil
}

// TODO share func
func writeConfig(path string, cfg *config.Config) error {
	filename, err := config.Filename(path)
	if err != nil {
		return err
	}
	return config.WriteConfigFile(filename, cfg)
}
