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
	Topic{
		ID:    ID("0"),
		Title: "Hello Mars",
		Text:  "Hello Mars",
	},
	Topic{
		ID:    ID("0.1"),
		Title: "Hello Mars 2",
		Text:  "Hello Mars 2",
	},
}
