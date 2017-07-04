package coredag

import (
	"fmt"
	"io"
	"io/ioutil"

	node "gx/ipfs/QmYNyRZJBUYPNrLszFmrBrPJbsBh2vMsefz5gnDpB5M1P6/go-ipld-format"
	ipldcbor "gx/ipfs/QmemYymP73eVdTUUMZEiSpiHeZQKNJdT5dP2iuHssZh1sR/go-ipld-cbor"
)

type DagParser func(r io.Reader) ([]node.Node, error)

type FormatParsers map[string]DagParser
type InputEncParsers map[string]FormatParsers

var DefaultInputEncParsers = InputEncParsers{
	"json": DefaultJsonParsers,
	"raw":  DefaultRawParsers,
}

var DefaultJsonParsers = FormatParsers{
	"cbor":     CborJsonParser,
	"dag-cbor": CborJsonParser,
}

var DefaultRawParsers = FormatParsers{
	"cbor":     CborRawParser,
	"dag-cbor": CborRawParser,
}

func ParseInputs(ienc, format string, r io.Reader) ([]node.Node, error) {
	return DefaultInputEncParsers.ParseInputs(ienc, format, r)
}

func (iep InputEncParsers) AddParser(ienv, format string, f DagParser) {
	m, ok := iep[ienv]
	if !ok {
		m = make(FormatParsers)
		iep[ienv] = m
	}

	m[format] = f
}

func (iep InputEncParsers) ParseInputs(ienc, format string, r io.Reader) ([]node.Node, error) {
	pset, ok := iep[ienc]
	if !ok {
		return nil, fmt.Errorf("no input parser for %q", ienc)
	}

	parser, ok := pset[format]
	if !ok {
		return nil, fmt.Errorf("no parser for format %q using input type %q", format, ienc)
	}

	return parser(r)
}

func CborJsonParser(r io.Reader) ([]node.Node, error) {
	nd, err := ipldcbor.FromJson(r)
	if err != nil {
		return nil, err
	}

	return []node.Node{nd}, nil
}

func CborRawParser(r io.Reader) ([]node.Node, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	nd, err := ipldcbor.Decode(data)
	if err != nil {
		return nil, err
	}

	return []node.Node{nd}, nil
}
