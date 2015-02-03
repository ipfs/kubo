package component

import (
	"io"

	"github.com/jbenet/go-ipfs/repo/config"
)

type Component interface {
	Open(*config.Config) error
	io.Closer
	SetPath(string)
}
type Initializer func(path string, conf *config.Config) error
type InitializationChecker func(path string) bool
