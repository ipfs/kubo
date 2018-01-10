package options

type PinAddSettings struct {
	Recursive bool
}

type PinLsSettings struct {
	Type string
}

type PinUpdateSettings struct {
	Unpin bool
}

type PinAddOption func(*PinAddSettings) error
type PinLsOption func(settings *PinLsSettings) error
type PinUpdateOption func(*PinUpdateSettings) error

func PinAddOptions(opts ...PinAddOption) (*PinAddSettings, error) {
	options := &PinAddSettings{
		Recursive: true,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}

	return options, nil
}

func PinLsOptions(opts ...PinLsOption) (*PinLsSettings, error) {
	options := &PinLsSettings{
		Type: "all",
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}

	return options, nil
}

func PinUpdateOptions(opts ...PinUpdateOption) (*PinUpdateSettings, error) {
	options := &PinUpdateSettings{
		Unpin: true,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}

	return options, nil
}

type PinOptions struct{}

func (api *PinOptions) WithRecursive(recucsive bool) PinAddOption {
	return func(settings *PinAddSettings) error {
		settings.Recursive = recucsive
		return nil
	}
}

func (api *PinOptions) WithType(t string) PinLsOption {
	return func(settings *PinLsSettings) error {
		settings.Type = t
		return nil
	}
}

func (api *PinOptions) WithUnpin(unpin bool) PinUpdateOption {
	return func(settings *PinUpdateSettings) error {
		settings.Unpin = unpin
		return nil
	}
}
