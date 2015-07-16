package config

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
)

var (
	// UseDefaultDatastoreErr is a sentinel value that lets caller
	// perform a special case open for the default datastore, with its
	// legacy requirements.
	UseDefaultDatastoreErr = errors.New("default datastore needs special handling")
)

type IPFSDatastore interface {
	datastore.Datastore // should be threadsafe, be careful
	Batch() (datastore.Batch, error)

	Close() error
}

// DSOpener is  an interface that lets the caller open a datastore.
//
// All values of DSOpener must have a JSON representation that is
// semantically equivalent to the Datastore section of the config
// file that they were instantiated by, including the "Type" key.
type DSOpener interface {
	Open(repoPath string) (IPFSDatastore, error)
}

type Datastore struct {
	ds DSOpener
}

var _ DSOpener = (*Datastore)(nil)

func (d *Datastore) Open(repoPath string) (IPFSDatastore, error) {
	if d.ds == nil {
		return nil, UseDefaultDatastoreErr
	}
	return d.ds.Open(repoPath)
}

var _ json.Unmarshaler = (*Datastore)(nil)

func (d *Datastore) UnmarshalJSON(data []byte) error {
	type envelope struct {
		Type string
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}

	switch env.Type {
	// "leveldb" here is for backwards compat; the content of that
	// section was not really used, so we ignore the path inside.
	case "default", "leveldb":
		// leave ds nil, it's handled in Open
		return nil

	case "s3":
		ds := s3Datastore{}
		if err := json.Unmarshal(data, &ds); err != nil {
			return fmt.Errorf("datastore s3: %v", err)
		}
		d.ds = ds
		return nil

	default:
		return fmt.Errorf("unsupported datastore type: %q", env.Type)
	}
}

var _ json.Marshaler = (*Datastore)(nil)

func (d *Datastore) MarshalJSON() ([]byte, error) {
	if d.ds != nil {
		return json.Marshal(d.ds)
	}
	return []byte(`{"Type": "default"}`), nil
}
