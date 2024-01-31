package options

type RoutingPutSettings struct {
	AllowOffline bool
}

type RoutingPutOption func(*RoutingPutSettings) error

func RoutingPutOptions(opts ...RoutingPutOption) (*RoutingPutSettings, error) {
	options := &RoutingPutSettings{
		AllowOffline: false,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}

	return options, nil
}

// nolint deprecated
// Deprecated: use [Routing] instead.
var Put = Routing

type RoutingProvideSettings struct {
	Recursive bool
}

type RoutingFindProvidersSettings struct {
	NumProviders int
}

type (
	RoutingProvideOption       func(*DhtProvideSettings) error
	RoutingFindProvidersOption func(*DhtFindProvidersSettings) error
)

func RoutingProvideOptions(opts ...RoutingProvideOption) (*RoutingProvideSettings, error) {
	options := &RoutingProvideSettings{
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

func RoutingFindProvidersOptions(opts ...RoutingFindProvidersOption) (*RoutingFindProvidersSettings, error) {
	options := &RoutingFindProvidersSettings{
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

type routingOpts struct{}

var Routing routingOpts

// Recursive is an option for [Routing.Provide] which specifies whether to provide
// the given path recursively.
func (routingOpts) Recursive(recursive bool) RoutingProvideOption {
	return func(settings *DhtProvideSettings) error {
		settings.Recursive = recursive
		return nil
	}
}

// NumProviders is an option for [Routing.FindProviders] which specifies the
// number of peers to look for. Default is 20.
func (routingOpts) NumProviders(numProviders int) RoutingFindProvidersOption {
	return func(settings *DhtFindProvidersSettings) error {
		settings.NumProviders = numProviders
		return nil
	}
}

// AllowOffline is an option for [Routing.Put] which specifies whether to allow
// publishing when the node is offline. Default value is false
func (routingOpts) AllowOffline(allow bool) RoutingPutOption {
	return func(settings *RoutingPutSettings) error {
		settings.AllowOffline = allow
		return nil
	}
}
