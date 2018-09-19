package options

import (
	"time"

	ropts "github.com/ipfs/go-ipfs/namesys/opts"
)

const (
	DefaultNameValidTime = 24 * time.Hour
)

type NamePublishSettings struct {
	ValidTime time.Duration
	Key       string
}

type NameResolveSettings struct {
	Local bool
	Cache bool

	ResolveOpts []ropts.ResolveOpt
}

type NamePublishOption func(*NamePublishSettings) error
type NameResolveOption func(*NameResolveSettings) error

func NamePublishOptions(opts ...NamePublishOption) (*NamePublishSettings, error) {
	options := &NamePublishSettings{
		ValidTime: DefaultNameValidTime,
		Key:       "self",
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}

	return options, nil
}

func NameResolveOptions(opts ...NameResolveOption) (*NameResolveSettings, error) {
	options := &NameResolveSettings{
		Local: false,
		Cache: true,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}

	return options, nil
}

type nameOpts struct{}

var Name nameOpts

// ValidTime is an option for Name.Publish which specifies for how long the
// entry will remain valid. Default value is 24h
func (nameOpts) ValidTime(validTime time.Duration) NamePublishOption {
	return func(settings *NamePublishSettings) error {
		settings.ValidTime = validTime
		return nil
	}
}

// Key is an option for Name.Publish which specifies the key to use for
// publishing. Default value is "self" which is the node's own PeerID.
// The key parameter must be either PeerID or keystore key alias.
//
// You can use KeyAPI to list and generate more names and their respective keys.
func (nameOpts) Key(key string) NamePublishOption {
	return func(settings *NamePublishSettings) error {
		settings.Key = key
		return nil
	}
}

// Local is an option for Name.Resolve which specifies if the lookup should be
// offline. Default value is false
func (nameOpts) Local(local bool) NameResolveOption {
	return func(settings *NameResolveSettings) error {
		settings.Local = local
		return nil
	}
}

// Cache is an option for Name.Resolve which specifies if cache should be used.
// Default value is true
func (nameOpts) Cache(cache bool) NameResolveOption {
	return func(settings *NameResolveSettings) error {
		settings.Cache = cache
		return nil
	}
}

//
func (nameOpts) ResolveOption(opt ropts.ResolveOpt) NameResolveOption {
	return func(settings *NameResolveSettings) error {
		settings.ResolveOpts = append(settings.ResolveOpts, opt)
		return nil
	}
}
