package peerlog

import (
	"fmt"
	"sync/atomic"
	"time"

	core "github.com/ipfs/go-ipfs/core"
	plugin "github.com/ipfs/go-ipfs/plugin"
	logging "github.com/ipfs/go-log"
	event "github.com/libp2p/go-libp2p-core/event"
	network "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"go.uber.org/zap"
)

var log = logging.Logger("plugin/peerlog")

type eventType int

var (
	// size of the event queue buffer
	eventQueueSize = 64 * 1024
	// number of events to drop when busy.
	busyDropAmount = eventQueueSize / 8
)

const (
	eventConnect eventType = iota
	eventIdentify
)

type plEvent struct {
	kind eventType
	peer peer.ID
}

// Log all the PeerIDs we see
//
// Usage:
//   GOLOG_FILE=~/peer.log IPFS_LOGGING_FMT=json ipfs daemon
// Output:
//   {"level":"info","ts":"2020-02-10T13:54:26.639Z","logger":"plugin/peerlog","caller":"peerlog/peerlog.go:51","msg":"connected","peer":"QmS2H72gdrekXJggGdE9SunXPntBqdkJdkXQJjuxcH8Cbt"}
//   {"level":"info","ts":"2020-02-10T13:54:59.095Z","logger":"plugin/peerlog","caller":"peerlog/peerlog.go:56","msg":"identified","peer":"QmS2H72gdrekXJggGdE9SunXPntBqdkJdkXQJjuxcH8Cbt","agent":"go-ipfs/0.5.0/"}
//
type peerLogPlugin struct {
	droppedCount uint64
	events       chan plEvent
}

var _ plugin.PluginDaemonInternal = (*peerLogPlugin)(nil)

// Plugins is exported list of plugins that will be loaded
var Plugins = []plugin.Plugin{
	&peerLogPlugin{},
}

// Name returns the plugin's name, satisfying the plugin.Plugin interface.
func (*peerLogPlugin) Name() string {
	return "peerlog"
}

// Version returns the plugin's version, satisfying the plugin.Plugin interface.
func (*peerLogPlugin) Version() string {
	return "0.1.0"
}

// Init initializes plugin
func (pl *peerLogPlugin) Init(*plugin.Environment) error {
	pl.events = make(chan plEvent, eventQueueSize)
	return nil
}

func (pl *peerLogPlugin) collectEvents(node *core.IpfsNode) {
	ctx := node.Context()

	busyCounter := 0
	dlog := log.Desugar()
	for {
		// Deal with dropped events.
		dropped := atomic.SwapUint64(&pl.droppedCount, 0)
		if dropped > 0 {
			busyCounter++

			// sleep a bit to give the system a chance to catch up with logging.
			select {
			case <-time.After(time.Duration(busyCounter) * time.Second):
			case <-ctx.Done():
				return
			}

			// drain 1/8th of the backlog backlog so we
			// don't immediately run into this situation
			// again.
		loop:
			for i := 0; i < busyDropAmount; i++ {
				select {
				case <-pl.events:
					dropped++
				default:
					break loop
				}
			}

			// Add in any events we've dropped in the mean-time.
			dropped += atomic.SwapUint64(&pl.droppedCount, 0)

			// Report that we've dropped events.
			dlog.Error("dropped events", zap.Uint64("count", dropped))
		} else {
			busyCounter = 0
		}

		var e plEvent
		select {
		case <-ctx.Done():
			return
		case e = <-pl.events:
		}

		peerID := zap.String("peer", e.peer.Pretty())

		switch e.kind {
		case eventConnect:
			dlog.Info("connected", peerID)
		case eventIdentify:
			agent, err := node.Peerstore.Get(e.peer, "AgentVersion")
			switch err {
			case nil:
			case peerstore.ErrNotFound:
				continue
			default:
				dlog.Error("failed to get agent version", zap.Error(err))
				continue
			}

			agentS, ok := agent.(string)
			if !ok {
				continue
			}
			dlog.Info("identified", peerID, zap.String("agent", agentS))
		}
	}
}

func (pl *peerLogPlugin) emit(evt eventType, p peer.ID) {
	select {
	case pl.events <- plEvent{kind: evt, peer: p}:
	default:
		atomic.AddUint64(&pl.droppedCount, 1)
	}
}

func (pl *peerLogPlugin) Start(node *core.IpfsNode) error {
	// Ensure logs from this plugin get printed regardless of global IPFS_LOGGING value
	if err := logging.SetLogLevel("plugin/peerlog", "info"); err != nil {
		return fmt.Errorf("failed to set log level: %w", err)
	}

	sub, err := node.PeerHost.EventBus().Subscribe(new(event.EvtPeerIdentificationCompleted))
	if err != nil {
		return fmt.Errorf("failed to subscribe to identify notifications")
	}

	var notifee network.NotifyBundle
	notifee.ConnectedF = func(net network.Network, conn network.Conn) {
		pl.emit(eventConnect, conn.RemotePeer())
	}
	node.PeerHost.Network().Notify(&notifee)

	go func() {
		defer sub.Close()
		for e := range sub.Out() {
			switch e := e.(type) {
			case event.EvtPeerIdentificationCompleted:
				pl.emit(eventIdentify, e.Peer)
			}
		}
	}()

	go pl.collectEvents(node)

	return nil
}

func (*peerLogPlugin) Close() error {
	return nil
}
