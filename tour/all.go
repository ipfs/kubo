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
