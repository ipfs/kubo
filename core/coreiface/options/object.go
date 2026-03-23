package options

type ObjectAddLinkSettings struct {
	Create               bool
	SkipUnixFSValidation bool
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

// SkipUnixFSValidation is an option for Object.AddLink which skips the check
// that only allows adding named links to UnixFS directory nodes.
// Use this when operating on raw dag-pb nodes outside of UnixFS semantics.
func (objectOpts) SkipUnixFSValidation(skip bool) ObjectAddLinkOption {
	return func(settings *ObjectAddLinkSettings) error {
		settings.SkipUnixFSValidation = skip
		return nil
	}
}

type ObjectRmLinkSettings struct {
	SkipUnixFSValidation bool
}

type (
	ObjectRmLinkOption func(*ObjectRmLinkSettings) error
)

func ObjectRmLinkOptions(opts ...ObjectRmLinkOption) (*ObjectRmLinkSettings, error) {
	options := &ObjectRmLinkSettings{}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}
	return options, nil
}

// RmLinkSkipUnixFSValidation is an option for Object.RmLink which skips the
// check that only allows removing links from UnixFS directory nodes.
// Use this when operating on raw dag-pb nodes outside of UnixFS semantics.
func (objectOpts) RmLinkSkipUnixFSValidation(skip bool) ObjectRmLinkOption {
	return func(settings *ObjectRmLinkSettings) error {
		settings.SkipUnixFSValidation = skip
		return nil
	}
}
