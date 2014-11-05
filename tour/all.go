package tour

import (
	"fmt"
	"sort"
)

func init() {
	for _, t := range allTopics {
		Topics[t.ID] = t
		IDs = append(IDs, t.ID)
	}

	sort.Sort(IDSlice(IDs))
}

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

// Chapter is used to define a chapter once and derive IDs for any number of
// sections within.
//
// eg.
// 		Intro := Chapter(1)
// 		ID("1.1") == Intro(1)
func Chapter(number int) func(topic int) ID {
	return func(topic int) ID {
		return ID(fmt.Sprintf("%d.%d", number, topic))
	}
}
