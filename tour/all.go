package tour

import (
	"sort"

	c "github.com/jbenet/go-ipfs/tour/content"
)

func init() {
	for _, t := range allTopics {
		Topics[t.ID] = t
		IDs = append(IDs, t.ID)
	}

	sort.Sort(IDSlice(IDs))
}

// Topics contains a mapping of Tour Topic ID to Topic
var allTopics = []Topic{
	Topic{
		ID:      ID("0.0"),
		Content: c.IntroHelloMars,
	},
	Topic{
		ID:      ID("0.1"),
		Content: c.IntroTour,
	},
	Topic{
		ID:      ID("0.2"),
		Content: c.IntroAboutIpfs,
	},
}
