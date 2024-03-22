package options

type ObjectAddLinkSettings struct {
	Create bool
}

type (
	ObjectAddLinkOption func(*ObjectAddLinkSettings) error
)

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

type objectOpts struct{}

var Object objectOpts

// Create is an option for Object.AddLink which specifies whether create required
// directories for the child
func (objectOpts) Create(create bool) ObjectAddLinkOption {
	return func(settings *ObjectAddLinkSettings) error {
		settings.Create = create
		return nil
	}
}
