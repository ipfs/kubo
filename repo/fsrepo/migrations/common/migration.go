// Package common contains common types and interfaces for file system repository migrations
package common

// Options contains migration options for embedded migrations
type Options struct {
	Path    string
	Verbose bool
}

// Migration is the interface that all migrations must implement
type Migration interface {
	Versions() string
	Apply(opts Options) error
	Revert(opts Options) error
	Reversible() bool
}
