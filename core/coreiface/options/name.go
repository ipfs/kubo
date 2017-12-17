package options

import (
	"time"
)

const (
	DefaultNameValidTime = 24 * time.Hour
)

type NamePublishSettings struct {
	ValidTime time.Duration
	Key       string
}

type NameResolveSettings struct {
	Recursive bool
	Local     bool
	Cache     bool
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
		Recursive: false,
		Local:     false,
		Cache:     true,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}

	return options, nil
}

type NameOptions struct{}

func (api *NameOptions) WithValidTime(validTime time.Duration) NamePublishOption {
	return func(settings *NamePublishSettings) error {
		settings.ValidTime = validTime
		return nil
	}
}

func (api *NameOptions) WithKey(key string) NamePublishOption {
	return func(settings *NamePublishSettings) error {
		settings.Key = key
		return nil
	}
}

func (api *NameOptions) WithRecursive(recursive bool) NameResolveOption {
	return func(settings *NameResolveSettings) error {
		settings.Recursive = recursive
		return nil
	}
}

func (api *NameOptions) WithLocal(local bool) NameResolveOption {
	return func(settings *NameResolveSettings) error {
		settings.Local = local
		return nil
	}
}

func (api *NameOptions) WithCache(cache bool) NameResolveOption {
	return func(settings *NameResolveSettings) error {
		settings.Cache = cache
		return nil
	}
}
