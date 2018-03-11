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

type pinOpts struct{}

var Pin pinOpts

// Recursive is an option for Pin.Add which specifies whether to pin an entire
// object tree or just one object. Default: true
func (_ pinOpts) Recursive(recucsive bool) PinAddOption {
	return func(settings *PinAddSettings) error {
		settings.Recursive = recucsive
		return nil
	}
}

// Type is an option for Pin.Ls which allows to specify which pin types should
// be returned
//
// Supported values:
// * "direct" - directly pinned objects
// * "recursive" - roots of recursive pins
// * "indirect" - indirectly pinned objects (referenced by recursively pinned
//    objects)
// * "all" - all pinned objects (default)
func (_ pinOpts) Type(t string) PinLsOption {
	return func(settings *PinLsSettings) error {
		settings.Type = t
		return nil
	}
}

// Unpin is an option for Pin.Update which specifies whether to remove the old pin.
// Default is true.
func (_ pinOpts) Unpin(unpin bool) PinUpdateOption {
	return func(settings *PinUpdateSettings) error {
		settings.Unpin = unpin
		return nil
	}
}
