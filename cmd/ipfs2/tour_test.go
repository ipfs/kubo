package main

import (
	"bytes"
	"testing"

	"github.com/jbenet/go-ipfs/tour"
)

func TestParseTourTemplate(t *testing.T) {
	topic := &tour.Topic{
		ID:    "42",
		Title: "IPFS CLI test files",
		Text: `Welcome to the IPFS test files
		This is where we test our beautiful command line interfaces
		`,
	}
	var buf bytes.Buffer
	err := tourShow(&buf, topic)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(buf.String())
}
