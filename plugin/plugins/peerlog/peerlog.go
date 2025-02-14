package peerlog

import (
	"fmt"
	"sync/atomic"
	"time"

	logging "github.com/ipfs/go-log"
	core "github.com/ipfs/kubo/core"
	plugin "github.com/ipfs/kubo/plugin"
	event "github.com/libp2p/go-libp2p/core/event"
	network "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"go.uber.org/zap"
)

var log = logging.Logger("plugin/peerlog")

type eventType int

var (
	// size of the event queue buffer.
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

// Log all the PeerIDs. This is considered internal, unsupported, and may break at any point.
//
// Usage:
//
//	GOLOG_FILE=~/peer.log GOLOG_LOG_FMT=json ipfs daemon
//
// Output:
//
//	{"level":"info","ts":"2020-02-10T13:54:26.639Z","logger":"plugin/peerlog","caller":"peerlog/peerlog.go:51","msg":"connected","peer":"QmS2H72gdrekXJggGdE9SunXPntBqdkJdkXQJjuxcH8Cbt"}
//	{"level":"info","ts":"2020-02-10T13:54:59.095Z","logger":"plugin/peerlog","caller":"peerlog/peerlog.go:56","msg":"identified","peer":"QmS2H72gdrekXJggGdE9SunXPntBqdkJdkXQJjuxcH8Cbt","agent":"go-ipfs/0.5.0/"}
type peerLogPlugin struct {
	enabled      bool
	droppedCount uint64
	events       chan plEvent
}

var _ plugin.PluginDaemonInternal = (*peerLogPlugin)(nil)

// Plugins is exported list of plugins that will be loaded.
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

// extractEnabled extracts the "Enabled" field from the plugin config.
// Do not follow this as a precedent, this is only applicable to this plugin,
// since it is internal-only, unsupported functionality.
// For supported functionality, we should rework the plugin API to support this use case
// of including plugins that are disabled by default.
func extractEnabled(config interface{}) bool {
	// plugin is disabled by default, unless Enabled=true
	if config == nil {
		return false
	}
	mapIface, ok := config.(map[string]interface{})
	if !ok {
		return false
	}
	enabledIface, ok := mapIface["Enabled"]
	if !ok || enabledIface == nil {
		return false
	}
	enabled, ok := enabledIface.(bool)
	if !ok {
		return false
	}
	return enabled
}

// Init initializes plugin.
func (pl *peerLogPlugin) Init(env *plugin.Environment) error {
	pl.events = make(chan plEvent, eventQueueSize)
	pl.enabled = extractEnabled(env.Config)
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

		peerID := zap.String("peer", e.peer.String())

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
	if !pl.enabled {
		return nil
	}

	// Ensure logs from this plugin get printed regardless of global GOLOG_LOG_LEVEL value
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
