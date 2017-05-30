package corenet

import (
	"io"

	ma "gx/ipfs/QmcyqRMCAXVtYPS4DiBrA7sezL9rRGfW8Ctx7cywL4TXJj/go-multiaddr"
	peer "gx/ipfs/QmdS9KpbDyPrieswibZhkod1oXqRwZJrUPzxCofAMWpFGq/go-libp2p-peer"
)

// AppInfo holds information on a local application protocol listener service.
type AppInfo struct {
	// Application protocol identifier.
	Protocol string

	// Node identity
	Identity peer.ID

	// Local protocol stream address.
	Address ma.Multiaddr

	// Local protocol stream listener.
	Closer io.Closer

	// Flag indicating whether we're still accepting incoming connections, or
	// whether this application listener has been shutdown.
	Running bool

	Registry *AppRegistry
}

func (c *AppInfo) Close() error {
	c.Registry.Deregister(c.Protocol)
	c.Closer.Close()
	return nil
}

// AppRegistry is a collection of local application protocol listeners.
type AppRegistry struct {
	Apps []*AppInfo
}

func (c *AppRegistry) Register(appInfo *AppInfo) {
	c.Apps = append(c.Apps, appInfo)
}

func (c *AppRegistry) Deregister(proto string) {
	foundAt := -1
	for i, a := range c.Apps {
		if a.Protocol == proto {
			foundAt = i
			break
		}
	}

	if foundAt != -1 {
		c.Apps = append(c.Apps[:foundAt], c.Apps[foundAt+1:]...)
	}
}
