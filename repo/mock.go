package repo

import (
	"errors"

	"github.com/ipfs/go-ipfs/repo/config"
)

var errTODO = errors.New("TODO")

// Mock is not thread-safe
type Mock struct {
	C config.Config
	D Datastore
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

func (m *Mock) Datastore() Datastore { return m.D }

func (m *Mock) SubDatastore(_ string) Datastore { return nil }

func (m *Mock) GetStorageUsage() (uint64, error) { return 0, nil }

func (m *Mock) Close() error { return errTODO }

func (m *Mock) SetAPIAddr(addr string) error { return errTODO }


