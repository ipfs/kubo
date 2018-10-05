package iface

import "errors"

var (
	ErrIsDir   = errors.New("object is a directory")
	ErrOffline = errors.New("this action must be run in online mode, try running 'ipfs daemon' first")
)
