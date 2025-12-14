package options

type DagImportSettings struct {
	PinRoots           bool
	PinRootsSet        bool
	Stats              bool
	StatsSet           bool
	FastProvideRoot    bool
	FastProvideRootSet bool
	FastProvideWait    bool
	FastProvideWaitSet bool
}

type DagImportOption func(*DagImportSettings) error

func DagImportOptions(opts ...DagImportOption) (*DagImportSettings, error) {
	options := &DagImportSettings{
		PinRoots:           false,
		PinRootsSet:        false,
		Stats:              false,
		StatsSet:           false,
		FastProvideRoot:    false,
		FastProvideRootSet: false,
		FastProvideWait:    false,
		FastProvideWaitSet: false,
	}

	for _, opt := range opts {
		err := opt(options)
		if err != nil {
			return nil, err
		}
	}

	return options, nil
}

type dagOpts struct{}

var Dag dagOpts

// PinRoots sets whether to pin roots listed in CAR headers after importing.
// If not set, server uses command default (true).
func (dagOpts) PinRoots(pin bool) DagImportOption {
	return func(settings *DagImportSettings) error {
		settings.PinRoots = pin
		settings.PinRootsSet = true
		return nil
	}
}

// Stats enables output of import statistics (block count and bytes).
// If not set, server uses command default (false).
func (dagOpts) Stats(enable bool) DagImportOption {
	return func(settings *DagImportSettings) error {
		settings.Stats = enable
		settings.StatsSet = true
		return nil
	}
}

// FastProvideRoot sets whether to immediately provide root CIDs to DHT for faster discovery.
// If not set, server uses Import.FastProvideRoot config value (default: true).
func (dagOpts) FastProvideRoot(enable bool) DagImportOption {
	return func(settings *DagImportSettings) error {
		settings.FastProvideRoot = enable
		settings.FastProvideRootSet = true
		return nil
	}
}

// FastProvideWait sets whether to block until fast provide completes.
// If not set, server uses Import.FastProvideWait config value (default: false).
func (dagOpts) FastProvideWait(enable bool) DagImportOption {
	return func(settings *DagImportSettings) error {
		settings.FastProvideWait = enable
		settings.FastProvideWaitSet = true
		return nil
	}
}
