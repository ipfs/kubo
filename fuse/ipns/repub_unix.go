package ipns

import "time"

type Republisher struct {
	TimeoutLong  time.Duration
	TimeoutShort time.Duration
	Publish      chan struct{}
	node         *Node
}

func NewRepublisher(n *Node, tshort, tlong time.Duration) *Republisher {
	return &Republisher{
		TimeoutShort: tshort,
		TimeoutLong:  tlong,
		Publish:      make(chan struct{}),
		node:         n,
	}
}

func (np *Republisher) Run() {
	for _ = range np.Publish {
		quick := time.After(np.TimeoutShort)
		longer := time.After(np.TimeoutLong)

	wait:
		select {
		case <-quick:
		case <-longer:
		case <-np.Publish:
			quick = time.After(np.TimeoutShort)
			goto wait
		}

		log.Info("Publishing Changes!")
		err := np.node.republishRoot()
		if err != nil {
			log.Critical("republishRoot error: %s", err)
		}

	}
}
