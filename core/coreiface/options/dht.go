package options

type DhtProvideSettings struct {
	Recursive bool
}

type DhtProvideOption func(*DhtProvideSettings) error

func DhtProvideOptions(opts ...DhtProvideOption) (*DhtProvideSettings, error) {
	options := &DhtProvideSettings{
		Recursive: false,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

type DhtOptions struct{}

func (api *DhtOptions) WithRecursive(recursive bool) DhtProvideOption {
	return func(settings *DhtProvideSettings) error {
		settings.Recursive = recursive
		return nil
	}
}
