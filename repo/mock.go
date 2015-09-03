package repo

import (
	"errors"

	ds "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	"github.com/ipfs/go-ipfs/repo/config"
)

var errTODO = errors.New("TODO")

// Mock is not thread-safe
type Mock struct {
	C config.Config
	D ds.ThreadSafeDatastore
}

func (m *Mock) Config() (*config.Config, error) {
	return &m.C, nil // FIXME threadsafety
}

func (m *Mock) SetConfig(updated *config.Config) error {
	m.C = *updated // FIXME threadsafety
	return nil
}

func (m *Mock) SetConfigKey(key string, value interface{}) error {
	return errTODO
}

func (m *Mock) GetConfigKey(key string) (interface{}, error) {
	return nil, errTODO
}

func (m *Mock) Datastore() ds.ThreadSafeDatastore { return m.D }

func (m *Mock) Close() error { return errTODO }

func (m *Mock) SetAPIAddr(addr string) error { return errTODO }
