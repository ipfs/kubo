package tour

import "sort"

func init() {
	for _, t := range allTopics {
		Topics[t.ID] = t
		IDs = append(IDs, t.ID)
	}

	sort.Sort(IDSlice(IDs))
}

// TODO move content into individual files if desired

// TODO(brian): If sub-topics are needed, write recursively (as tree comprised
// of Section nodes:
//
// type Section interface {
// 	Sections() []Section
// 	Topic() Topic
// }

var (
	Introduction = Chapter(0)
	FileBasics   = Chapter(1)
	NodeBasics   = Chapter(2)
	MerkleDag    = Chapter(3)
	Network      = Chapter(4)
	Daemon       = Chapter(5)
	Routing      = Chapter(6)
	Exchange     = Chapter(7)
	Ipns         = Chapter(8)
	Mounting     = Chapter(9)
	Plumbing     = Chapter(10)
	Formats      = Chapter(11)
)

// Topics contains a mapping of Tour Topic ID to Topic
var allTopics = []Topic{
	Topic{ID: Introduction(0), Content: IntroHelloMars},
	Topic{ID: Introduction(1), Content: IntroTour},
	Topic{ID: Introduction(2), Content: IntroAboutIpfs},

	Topic{ID: FileBasics(1), Content: FileBasicsFilesystem},
	Topic{ID: FileBasics(2), Content: FileBasicsGetting},
	Topic{ID: FileBasics(3), Content: FileBasicsAdding},
	Topic{ID: FileBasics(4), Content: FileBasicsDirectories},
	Topic{ID: FileBasics(5), Content: FileBasicsDistributed},
	Topic{ID: FileBasics(6), Content: FileBasicsMounting},

	Topic{NodeBasics(0), NodeBasicsInit},
	Topic{NodeBasics(1), NodeBasicsHelp},
	Topic{NodeBasics(2), NodeBasicsUpdate},
	Topic{NodeBasics(3), NodeBasicsConfig},

	Topic{MerkleDag(0), MerkleDagIntro},
	Topic{MerkleDag(1), MerkleDagContentAddressing},
	Topic{MerkleDag(2), MerkleDagContentAddressingLinks},
	Topic{MerkleDag(3), MerkleDagRedux},
	Topic{MerkleDag(4), MerkleDagIpfsObjects},
	Topic{MerkleDag(5), MerkleDagIpfsPaths},
	Topic{MerkleDag(6), MerkleDagImmutability},
	Topic{MerkleDag(7), MerkleDagUseCaseUnixFS},
	Topic{MerkleDag(8), MerkleDagUseCaseGitObjects},
	Topic{MerkleDag(9), MerkleDagUseCaseOperationalTransforms},

	Topic{Network(0), Network_Intro},
	Topic{Network(1), Network_Ipfs_Peers},
	Topic{Network(2), Network_Daemon},
	Topic{Network(3), Network_Routing},
	Topic{Network(4), Network_Exchange},
	Topic{Network(5), Network_Intro},

	Topic{Daemon(0), Daemon_Intro},
	Topic{Daemon(1), Daemon_Running_Commands},
	Topic{Daemon(2), Daemon_Web_UI},

	Topic{Routing(0), Routing_Intro},
	Topic{Routing(1), Rouing_Interface},
	Topic{Routing(2), Routing_Resolving},
	Topic{Routing(3), Routing_DHT},
	Topic{Routing(4), Routing_Other},

	Topic{Exchange(0), Exchange_Intro},
	Topic{Exchange(1), Exchange_Getting_Blocks},
	Topic{Exchange(2), Exchange_Strategies},
	Topic{Exchange(3), Exchange_Bitswap},

	Topic{Ipns(0), Ipns_Name_System},
	Topic{Ipns(1), Ipns_Mutability},
	Topic{Ipns(2), Ipns_PKI_Review},
	Topic{Ipns(3), Ipns_Publishing},
	Topic{Ipns(4), Ipns_Resolving},
	Topic{Ipns(5), Ipns_Consistency},
	Topic{Ipns(6), Ipns_Records_Etc},

	Topic{Mounting(0), Mounting_General},
	Topic{Mounting(1), Mounting_Ipfs},
	Topic{Mounting(2), Mounting_Ipns},

	Topic{Plumbing(0), Plumbing_Intro},
	Topic{Plumbing(1), Plumbing_Ipfs_Block},
	Topic{Plumbing(2), Plumbing_Ipfs_Object},
	Topic{Plumbing(3), Plumbing_Ipfs_Refs},
	Topic{Plumbing(4), Plumbing_Ipfs_Ping},
	Topic{Plumbing(5), Plumbing_Ipfs_Id},

	Topic{Formats(0), Formats_MerkleDag},
	Topic{Formats(1), Formats_Multihash},
	Topic{Formats(2), Formats_Multiaddr},
	Topic{Formats(3), Formats_Multicodec},
	Topic{Formats(4), Formats_Multicodec},
	Topic{Formats(5), Formats_Multikey},
	Topic{Formats(6), Formats_Protocol_Specific},
}

// Introduction

var IntroHelloMars = Content{
	Title: "Hello Mars",
	Text: `
	check things work
	`,
}
var IntroTour = Content{
	Title: "Hello Mars",
	Text: `
	how this works
	`,
}
var IntroAboutIpfs = Content{
	Title: "About IPFS",
}

// File Basics

var FileBasicsFilesystem = Content{
	Title: "Filesystem",
	Text: `
	`,
}
var FileBasicsGetting = Content{
	Title: "Getting Files",
	Text: `ipfs cat
	`,
}
var FileBasicsAdding = Content{
	Title: "Adding Files",
	Text: `ipfs add
	`,
}
var FileBasicsDirectories = Content{
	Title: "Directories",
	Text: `ipfs ls
	`,
}
var FileBasicsDistributed = Content{
	Title: "Distributed",
	Text: `ipfs cat from mars
	`,
}
var FileBasicsMounting = Content{
	Title: "Getting Files",
	Text: `ipfs mount (simple)
	`,
}

// Node Basics

var NodeBasicsInit = Content{
	Title: "Basics - init",
	Text: `
	`,
}
var NodeBasicsHelp = Content{
	Title: "Basics - help",
	Text: `
	`,
}
var NodeBasicsUpdate = Content{
	Title: "Basics - update",
	Text: `
	`,
}
var NodeBasicsConfig = Content{
	Title: "Basics - config",
	Text: `
	`,
}

// Merkle DAG
var MerkleDagIntro = Content{}
var MerkleDagContentAddressing = Content{}
var MerkleDagContentAddressingLinks = Content{}
var MerkleDagRedux = Content{}
var MerkleDagIpfsObjects = Content{}
var MerkleDagIpfsPaths = Content{}
var MerkleDagImmutability = Content{}
var MerkleDagUseCaseUnixFS = Content{}
var MerkleDagUseCaseGitObjects = Content{}
var MerkleDagUseCaseOperationalTransforms = Content{}

var Network_Intro = Content{}
var Network_Ipfs_Peers = Content{}
var Network_Daemon = Content{}
var Network_Routing = Content{}
var Network_Exchange = Content{}
var Network_Naming = Content{}

var Daemon_Intro = Content{}
var Daemon_Running_Commands = Content{}
var Daemon_Web_UI = Content{}

var Routing_Intro = Content{}
var Rouing_Interface = Content{}
var Routing_Resolving = Content{}
var Routing_DHT = Content{}
var Routing_Other = Content{}

var Exchange_Intro = Content{}
var Exchange_Bitswap = Content{}
var Exchange_Strategies = Content{}
var Exchange_Getting_Blocks = Content{}

var Ipns_Consistency = Content{}
var Ipns_Mutability = Content{}
var Ipns_Name_System = Content{}
var Ipns_PKI_Review = Content{}
var Ipns_Publishing = Content{}
var Ipns_Records_Etc = Content{}
var Ipns_Resolving = Content{}

var Mounting_General = Content{} // TODO note fuse
var Mounting_Ipfs = Content{}    // TODO cd, ls, cat
var Mounting_Ipns = Content{}    // TODO editing

var Plumbing_Intro = Content{}
var Plumbing_Ipfs_Block = Content{}
var Plumbing_Ipfs_Object = Content{}
var Plumbing_Ipfs_Refs = Content{}
var Plumbing_Ipfs_Ping = Content{}
var Plumbing_Ipfs_Id = Content{}

var Formats_MerkleDag = Content{}
var Formats_Multihash = Content{}
var Formats_Multiaddr = Content{}
var Formats_Multicodec = Content{}
var Formats_Multikey = Content{}
var Formats_Protocol_Specific = Content{}
