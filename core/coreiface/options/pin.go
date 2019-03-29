package options

type PinAddSettings struct {
	Recursive bool
}

type PinLsSettings struct {
	Type string
}

// PinRmSettings represents the settings of pin rm command
type PinRmSettings struct {
	Recursive bool
}

type PinUpdateSettings struct {
	Unpin bool
}

type PinAddOption func(*PinAddSettings) error

// PinRmOption pin rm option func
type PinRmOption func(*PinRmSettings) error

// PinLsOption pin ls option func
type PinLsOption func(*PinLsSettings) error
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

// PinRmOptions pin rm options
func PinRmOptions(opts ...PinRmOption) (*PinRmSettings, error) {
	options := &PinRmSettings{
		Recursive: true,
	}

	for _, opt := range opts {
		if err := opt(options); err != nil {
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

type pinType struct{}

type pinOpts struct {
	Type pinType
}

var Pin pinOpts

// All is an option for Pin.Ls which will make it return all pins. It is
// the default
func (pinType) All() PinLsOption {
	return Pin.pinType("all")
}

// Recursive is an option for Pin.Ls which will make it only return recursive
// pins
func (pinType) Recursive() PinLsOption {
	return Pin.pinType("recursive")
}

// Direct is an option for Pin.Ls which will make it only return direct (non
// recursive) pins
func (pinType) Direct() PinLsOption {
	return Pin.pinType("direct")
}

// Indirect is an option for Pin.Ls which will make it only return indirect pins
// (objects referenced by other recursively pinned objects)
func (pinType) Indirect() PinLsOption {
	return Pin.pinType("indirect")
}

// Recursive is an option for Pin.Add which specifies whether to pin an entire
// object tree or just one object. Default: true
func (pinOpts) Recursive(recursive bool) PinAddOption {
	return func(settings *PinAddSettings) error {
		settings.Recursive = recursive
		return nil
	}
}

// RmRecursive is an option for Pin.Rm which specifies whether to recursively
// unpin the object linked to by the specified object(s). This does not remove
// indirect pins referenced by other recursive pins.
func (pinOpts) RmRecursive(recursive bool) PinRmOption {
	return func(settings *PinRmSettings) error {
		settings.Recursive = recursive
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
func (pinOpts) pinType(t string) PinLsOption {
	return func(settings *PinLsSettings) error {
		settings.Type = t
		return nil
	}
}

// Unpin is an option for Pin.Update which specifies whether to remove the old pin.
// Default is true.
func (pinOpts) Unpin(unpin bool) PinUpdateOption {
	return func(settings *PinUpdateSettings) error {
		settings.Unpin = unpin
		return nil
	}
}
