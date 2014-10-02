package ipns

import "time"

type Republisher struct {
	Timeout time.Duration
	Publish chan struct{}
	node    *Node
}

func NewRepublisher(n *Node, tout time.Duration) *Republisher {
	return &Republisher{
		Timeout: tout,
		Publish: make(chan struct{}),
		node:    n,
	}
}

func (np *Republisher) Run() {
	for _ = range np.Publish {
		timer := time.After(np.Timeout)
		for {
			select {
			case <-timer:
				//Do the publish!
				log.Info("Publishing Changes!")
				err := np.node.updateTree()
				if err != nil {
					log.Critical("updateTree error: %s", err)
				}
				goto done
			case <-np.Publish:
				timer = time.After(np.Timeout)
			}
		}
	done:
	}
}
