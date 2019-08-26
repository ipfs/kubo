package p9client

type Options struct {
	network, address, version string
	msize                     int
}

type Option func(*Options)

func Address(address string) Option {
	return func(ops *Options) {
		ops.address = address
	}
}

func Version(version string) Option {
	return func(ops *Options) {
		ops.version = version
	}
}

func Msize(msize int) Option {
	return func(ops *Options) {
		ops.msize = msize
	}
}
