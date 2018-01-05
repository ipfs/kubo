package options

type ObjectNewSettings struct {
	Type string
}

type ObjectPutSettings struct {
	InputEnc string
	DataType string
}

type ObjectAddLinkSettings struct {
	Create bool
}

type ObjectNewOption func(*ObjectNewSettings) error
type ObjectPutOption func(*ObjectPutSettings) error
type ObjectAddLinkOption func(*ObjectAddLinkSettings) error

func ObjectNewOptions(opts ...ObjectNewOption) (*ObjectNewSettings, error) {
	options := &ObjectNewSettings{
		Type: "empty",
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

func ObjectPutOptions(opts ...ObjectPutOption) (*ObjectPutSettings, error) {
	options := &ObjectPutSettings{
		InputEnc: "json",
		DataType: "text",
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

func ObjectAddLinkOptions(opts ...ObjectAddLinkOption) (*ObjectAddLinkSettings, error) {
	options := &ObjectAddLinkSettings{
		Create: false,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

type ObjectOptions struct{}

func (api *ObjectOptions) WithType(t string) ObjectNewOption {
	return func(settings *ObjectNewSettings) error {
		settings.Type = t
		return nil
	}
}

func (api *ObjectOptions) WithInputEnc(e string) ObjectPutOption {
	return func(settings *ObjectPutSettings) error {
		settings.InputEnc = e
		return nil
	}
}

func (api *ObjectOptions) WithDataType(t string) ObjectPutOption {
	return func(settings *ObjectPutSettings) error {
		settings.DataType = t
		return nil
	}
}

func (api *ObjectOptions) WithCreate(create bool) ObjectAddLinkOption {
	return func(settings *ObjectAddLinkSettings) error {
		settings.Create = create
		return nil
	}
}
