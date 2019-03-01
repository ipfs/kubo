package fusemount

type rootBase struct {
	recordBase
	//mountTime *fuse.Timespec
}

//type rootList [tRoots]fusePath
type mountRoot struct {
	rootBase
	//rootIndices rootList
}

//should inherit from directory entries
type ipfsRoot struct {
	rootBase
	//TODO: review below
	//sync.Mutex
	//subnodes    []fuseStatPair
	//lastUpdated time.Time
}

type ipnsRoot struct {
	rootBase
	//sync.Mutex
	//keys []coreiface.Key
	//subnodes    []fuseStatPair
	//lastUpdated time.Time
}

type mfsRoot struct {
	rootBase
}
