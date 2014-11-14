package main

import (
	"bytes"
	"errors"
	"testing"

	"github.com/jbenet/go-ipfs/tour"
)

func TestParseTourTemplate(t *testing.T) {
	topic := &tour.Topic{
		ID:    "42",
		Title: "IPFS CLI test files",
		Text: `
Welcome to the IPFS test files
This is where we test our beautiful command line interfaces
		`,
	}
	var buf bytes.Buffer
	err := fprintTourShow(&buf, topic)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(buf.String())
}

func TestRenderTourOutputList(t *testing.T) {

	t.Log(`Ensure we can successfully print the tour output when there's an
	error and list of tour topics`)
	listOutput := &tourOutput{
		Error: errors.New("Topic 42 does not exist"),
		Topics: []tour.Topic{
			tour.Topic{
				ID:    "41",
				Title: "Being one shy of the mark",
				Text:  "Poor thing.",
			},
			tour.Topic{
				ID:    "44",
				Title: "Two shy of the mark",
				Text:  "Oh no.",
			},
		},
	}

	var list bytes.Buffer
	if err := printTourOutput(&list, listOutput); err != nil {
		t.Fatal(err)
	}
	t.Log(list.String())
}

func TestRenderTourOutputSingle(t *testing.T) {
	t.Log(`
	When there's just a single topic in the output, ensure we can render the
	template`)
	singleOutput := &tourOutput{
		Topic: &tour.Topic{
			ID:    "42",
			Title: "Informative!",
			Text:  "Compelling!",
		},
	}
	var single bytes.Buffer
	if err := printTourOutput(&single, singleOutput); err != nil {
		t.Fatal(err)
	}
	t.Log(single.String())
}
