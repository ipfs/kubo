package options

import (
	"time"
)

type NamePublishSettings struct {
	ValidTime time.Duration
	Key       string
}

type NameResolveSettings struct {
	Recursive bool
	Local     bool
	Nocache   bool
}

type NamePublishOption func(*NamePublishSettings) error
type NameResolveOption func(*NameResolveSettings) error

func NamePublishOptions(opts ...NamePublishOption) (*NamePublishSettings, error) {
	options := &NamePublishSettings{
		ValidTime: 24 * time.Hour,
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
		Nocache:   false,
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

func (api *NameOptions) WithNoCache(nocache bool) NameResolveOption {
	return func(settings *NameResolveSettings) error {
		settings.Nocache = nocache
		return nil
	}
}
