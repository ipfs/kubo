package tour

import "sort"

func init() {
	for _, t := range allTopics {
		Topics[t.ID] = t
		IDs = append(IDs, t.ID)
	}

	sort.Sort(IDSlice(IDs))
}

// Topics contains a mapping of Tour Topic ID to Topic
var allTopics = []Topic{
	Topic{ID: ID("0.0"), Content: IntroHelloMars},
	Topic{ID: ID("0.1"), Content: IntroTour},
	Topic{ID: ID("0.2"), Content: IntroAboutIpfs},

	// File Basics
	Topic{ID: ID("X.0"), Content: FileBasicsFilesystem},
	Topic{ID: ID("X.1"), Content: FileBasicsGetting},
	Topic{ID: ID("X.2"), Content: FileBasicsAdding},
	Topic{ID: ID("X.3"), Content: FileBasicsDirectories},
	Topic{ID: ID("X.3"), Content: FileBasicsDirectories},
	Topic{ID: ID("X.4"), Content: FileBasicsMounting},
	Topic{ID: ID("X.2"), Content: FileBasicsAdding},
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
