package options

type DhtProvideSettings struct {
	Recursive bool
}

type DhtFindProvidersSettings struct {
	NumProviders int
}

type DhtProvideOption func(*DhtProvideSettings) error
type DhtFindProvidersOption func(*DhtFindProvidersSettings) error

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

func DhtFindProvidersOptions(opts ...DhtFindProvidersOption) (*DhtFindProvidersSettings, error) {
	options := &DhtFindProvidersSettings{
		NumProviders: 20,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

type dhtOpts struct{}

var Dht dhtOpts

// WithRecursive is an option for Dht.Provide which specifies whether to provide
// the given path recursively
func (dhtOpts) WithRecursive(recursive bool) DhtProvideOption {
	return func(settings *DhtProvideSettings) error {
		settings.Recursive = recursive
		return nil
	}
}

// WithNumProviders is an option for Dht.FindProviders which specifies the
// number of peers to look for. Default is 20
func (dhtOpts) WithNumProviders(numProviders int) DhtFindProvidersOption {
	return func(settings *DhtFindProvidersSettings) error {
		settings.NumProviders = numProviders
		return nil
	}
}
