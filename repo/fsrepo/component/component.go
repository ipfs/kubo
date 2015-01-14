package component

import (
	"io"

	"github.com/jbenet/go-ipfs/repo/config"
)

type Component interface {
	Open() error
	io.Closer
}
type Initializer func(path string, conf *config.Config) error
type InitializationChecker func(path string) bool
