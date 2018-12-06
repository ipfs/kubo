package options

type ApiSettings struct {
	Offline bool
}

type ApiOption func(*ApiSettings) error

func ApiOptions(opts ...ApiOption) (*ApiSettings, error) {
	options := &ApiSettings{
		Offline: false,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

type apiOpts struct{}

var Api dagOpts

func (dagOpts) Offline(offline bool) ApiOption {
	return func(settings *ApiSettings) error {
		settings.Offline = offline
		return nil
	}
}
