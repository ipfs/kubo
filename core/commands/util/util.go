// Package util provides utilities shared amongst various commands
package util

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	cmds "github.com/jbenet/go-ipfs/thirdparty/commands"
	core "github.com/jbenet/go-ipfs/core"
	dag "github.com/jbenet/go-ipfs/struct/merkledag"
	u "github.com/jbenet/go-ipfs/util"
)

// ProgressBarMinSize defines the minimum output amount before showing a progress bar.
const ProgressBarMinSize = 1024 * 1024 * 4 // show progress bar for outputs > 4MiB

// ErrNotOnline signals a command must be run in online mode.
var ErrNotOnline = errors.New("This command must be run in online mode. Try running 'ipfs daemon' first.")

type Object struct {
	Hash  string
	Links []Link
}

// Link is a shared command output type
type Link struct {
	Name string
	Hash string
	Size uint64
}

// KeyList is a general type for outputting lists of keys
type KeyList struct {
	Keys []u.Key
}

type MessageOutput struct {
	Message string
}

func MessageTextMarshaler(res cmds.Response) (io.Reader, error) {
	return strings.NewReader(res.Output().(*MessageOutput).Message), nil
}

// KeyListTextMarshaler outputs a KeyList as plaintext, one key per line
func KeyListTextMarshaler(res cmds.Response) (io.Reader, error) {
	output := res.Output().(*KeyList)
	var buf bytes.Buffer
	for _, key := range output.Keys {
		buf.WriteString(key.B58String() + "\n")
	}
	return &buf, nil
}

func NodeOutput(dagnode *dag.Node) (*Object, error) {
	key, err := dagnode.Key()
	if err != nil {
		return nil, err
	}

	output := &Object{
		Hash:  key.Pretty(),
		Links: make([]Link, len(dagnode.Links)),
	}

	for i, link := range dagnode.Links {
		output.Links[i] = Link{
			Name: link.Name,
			Hash: link.Hash.B58String(),
			Size: link.Size,
		}
	}

	return output, nil
}

func MarshalLinks(links []Link) (s string) {
	for _, link := range links {
		s += fmt.Sprintf("%s %v %s\n", link.Hash, link.Size, link.Name)
	}
	return s
}

func AddNode(n *core.IpfsNode, node *dag.Node) error {
	err := n.DAG.AddRecursive(node) // add the file to the graph + local storage
	if err != nil {
		return err
	}

	err = n.Pinning.Pin(node, true) // ensure we keep it
	if err != nil {
		return err
	}

	return nil
}
