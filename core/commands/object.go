package commands

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/jbenet/go-ipfs/core"
	dag "github.com/jbenet/go-ipfs/merkledag"
)

// ObjectData takes a key string from args and writes out the raw bytes of that node (if there is one)
func ObjectData(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	dagnode, err := n.Resolver.ResolvePath(args[0])
	if err != nil {
		return fmt.Errorf("objectData error: %v", err)
	}
	log.Debug("objectData: found dagnode %q (# of bytes: %d - # links: %d)", args[0], len(dagnode.Data), len(dagnode.Links))

	_, err = io.Copy(out, bytes.NewReader(dagnode.Data))
	return err
}

// ObjectLinks takes a key string from args and lists the links it points to
func ObjectLinks(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	dagnode, err := n.Resolver.ResolvePath(args[0])
	if err != nil {
		return fmt.Errorf("objectLinks error: %v", err)
	}
	log.Debug("ObjectLinks: found dagnode %q (# of bytes: %d - # links: %d)", args[0], len(dagnode.Data), len(dagnode.Links))

	for _, link := range dagnode.Links {
		_, err = fmt.Fprintf(out, "%s %d %q\n", link.Hash.B58String(), link.Size, link.Name)
		if err != nil {
			break
		}
	}

	return err
}

// ErrUnknownObjectEnc is returned if a invalid encoding is supplied
var ErrUnknownObjectEnc = errors.New("unknown object encoding")

type objectEncoding string

const (
	objectEncodingJSON     objectEncoding = "json"
	objectEncodingProtobuf                = "protobuf"
)

func getObjectEnc(o interface{}) objectEncoding {
	v, ok := o.(string)
	if !ok {
		// chosen as default because it's human readable
		log.Warning("option is not a string - falling back to json")
		return objectEncodingJSON
	}

	return objectEncoding(v)
}

// ObjectGet takes a key string from args and a format option and serializes the dagnode to that format
func ObjectGet(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	dagnode, err := n.Resolver.ResolvePath(args[0])
	if err != nil {
		return fmt.Errorf("ObjectGet error: %v", err)
	}
	log.Debug("objectGet: found dagnode %q (# of bytes: %d - # links: %d)", args[0], len(dagnode.Data), len(dagnode.Links))

	// sadly all encodings dont implement a common interface
	var data []byte
	switch getObjectEnc(opts["encoding"]) {
	case objectEncodingJSON:
		data, err = json.MarshalIndent(dagnode, "", "  ")

	case objectEncodingProtobuf:
		data, err = dagnode.Marshal()

	default:
		return ErrUnknownObjectEnc
	}

	if err != nil {
		return fmt.Errorf("ObjectGet error: %v", err)
	}

	_, err = io.Copy(out, bytes.NewReader(data))
	return err
}

// ObjectPut takes a format option, serilizes bytes from stdin and updates the dag with that data
func ObjectPut(n *core.IpfsNode, args []string, opts map[string]interface{}, out io.Writer) error {
	var (
		dagnode *dag.Node
		data    []byte
		err     error
	)

	data, err = ioutil.ReadAll(io.LimitReader(os.Stdin, 512*1024))
	if err != nil {
		return fmt.Errorf("ObjectPut error: %v", err)
	}

	switch getObjectEnc(opts["encoding"]) {
	case objectEncodingJSON:
		dagnode = new(dag.Node)
		err = json.Unmarshal(data, dagnode)

	case objectEncodingProtobuf:
		dagnode, err = dag.Decoded(data)

	default:
		return ErrUnknownObjectEnc
	}

	if err != nil {
		return fmt.Errorf("ObjectPut error: %v", err)
	}

	return addNode(n, dagnode, "stdin", out)
}
