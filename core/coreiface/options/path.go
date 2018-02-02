package options

type ParsePathSettings struct {
	Resolve bool
}

type ParsePathOption func(*ParsePathSettings) error

func ParsePathOptions(opts ...ParsePathOption) (*ParsePathSettings, error) {
	options := &ParsePathSettings{
		Resolve: false,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

type ApiOptions struct{}

func (api *ApiOptions) WithResolve(r bool) ParsePathOption {
	return func(settings *ParsePathSettings) error {
		settings.Resolve = r
		return nil
	}
}
