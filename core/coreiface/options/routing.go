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

type putOpts struct{}

var Put putOpts

// AllowOffline is an option for Routing.Put which specifies whether to allow
// publishing when the node is offline. Default value is false
func (putOpts) AllowOffline(allow bool) RoutingPutOption {
	return func(settings *RoutingPutSettings) error {
		settings.AllowOffline = allow
		return nil
	}
}
