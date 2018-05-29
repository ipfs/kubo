package iface

import (
	"io"
)

type Reader interface {
	io.ReadSeeker
	io.Closer
}
