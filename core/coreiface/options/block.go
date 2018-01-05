package options

type BlockPutSettings struct {
	Codec    uint64
	MhType   uint64
	MhLength int
}

type BlockRmSettings struct {
	Force bool
}

type BlockPutOption func(*BlockPutSettings) error
type BlockRmOption func(*BlockRmSettings) error
