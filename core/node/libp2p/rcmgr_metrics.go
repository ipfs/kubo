package libp2p

import (
	"errors"
	"strconv"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"

	"github.com/prometheus/client_golang/prometheus"
)

func mustRegister(c prometheus.Collector) {
	err := prometheus.Register(c)
	are := prometheus.AlreadyRegisteredError{}
	if errors.As(err, &are) {
		return
	}
	if err != nil {
		panic(err)
	}
}

func createRcmgrMetrics() rcmgr.MetricsReporter {
	const (
		direction = "direction"
		usesFD    = "usesFD"
		protocol  = "protocol"
		service   = "service"
	)

	connAllowed := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_conns_allowed_total",
			Help: "allowed connections",
		},
		[]string{direction, usesFD},
	)
	mustRegister(connAllowed)

	connBlocked := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_conns_blocked_total",
			Help: "blocked connections",
		},
		[]string{direction, usesFD},
	)
	mustRegister(connBlocked)

	streamAllowed := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_streams_allowed_total",
			Help: "allowed streams",
		},
		[]string{direction},
	)
	mustRegister(streamAllowed)

	streamBlocked := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_streams_blocked_total",
			Help: "blocked streams",
		},
		[]string{direction},
	)
	mustRegister(streamBlocked)

	peerAllowed := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_peers_allowed_total",
		Help: "allowed peers",
	})
	mustRegister(peerAllowed)

	peerBlocked := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_peer_blocked_total",
		Help: "blocked peers",
	})
	mustRegister(peerBlocked)

	protocolAllowed := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_protocols_allowed_total",
			Help: "allowed streams attached to a protocol",
		},
		[]string{protocol},
	)
	mustRegister(protocolAllowed)

	protocolBlocked := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_protocols_blocked_total",
			Help: "blocked streams attached to a protocol",
		},
		[]string{protocol},
	)
	mustRegister(protocolBlocked)

	protocolPeerBlocked := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_protocols_for_peer_blocked_total",
			Help: "blocked streams attached to a protocol for a specific peer",
		},
		[]string{protocol},
	)
	mustRegister(protocolPeerBlocked)

	serviceAllowed := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_services_allowed_total",
			Help: "allowed streams attached to a service",
		},
		[]string{service},
	)
	mustRegister(serviceAllowed)

	serviceBlocked := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_services_blocked_total",
			Help: "blocked streams attached to a service",
		},
		[]string{service},
	)
	mustRegister(serviceBlocked)

	servicePeerBlocked := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "libp2p_rcmgr_service_for_peer_blocked_total",
			Help: "blocked streams attached to a service for a specific peer",
		},
		[]string{service},
	)
	mustRegister(servicePeerBlocked)

	memoryAllowed := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_memory_allocations_allowed_total",
		Help: "allowed memory allocations",
	})
	mustRegister(memoryAllowed)

	memoryBlocked := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "libp2p_rcmgr_memory_allocations_blocked_total",
		Help: "blocked memory allocations",
	})
	mustRegister(memoryBlocked)

	return rcmgrMetrics{
		connAllowed,
		connBlocked,
		streamAllowed,
		streamBlocked,
		peerAllowed,
		peerBlocked,
		protocolAllowed,
		protocolBlocked,
		protocolPeerBlocked,
		serviceAllowed,
		serviceBlocked,
		servicePeerBlocked,
		memoryAllowed,
		memoryBlocked,
	}
}

// Failsafe to ensure interface from go-libp2p-resource-manager is implemented
var _ rcmgr.MetricsReporter = rcmgrMetrics{}

type rcmgrMetrics struct {
	connAllowed         *prometheus.CounterVec
	connBlocked         *prometheus.CounterVec
	streamAllowed       *prometheus.CounterVec
	streamBlocked       *prometheus.CounterVec
	peerAllowed         prometheus.Counter
	peerBlocked         prometheus.Counter
	protocolAllowed     *prometheus.CounterVec
	protocolBlocked     *prometheus.CounterVec
	protocolPeerBlocked *prometheus.CounterVec
	serviceAllowed      *prometheus.CounterVec
	serviceBlocked      *prometheus.CounterVec
	servicePeerBlocked  *prometheus.CounterVec
	memoryAllowed       prometheus.Counter
	memoryBlocked       prometheus.Counter
}

func getDirection(d network.Direction) string {
	switch d {
	default:
		return ""
	case network.DirInbound:
		return "inbound"
	case network.DirOutbound:
		return "outbound"
	}
}

func (r rcmgrMetrics) AllowConn(dir network.Direction, usefd bool) {
	r.connAllowed.WithLabelValues(getDirection(dir), strconv.FormatBool(usefd)).Inc()
}

func (r rcmgrMetrics) BlockConn(dir network.Direction, usefd bool) {
	r.connBlocked.WithLabelValues(getDirection(dir), strconv.FormatBool(usefd)).Inc()
}

func (r rcmgrMetrics) AllowStream(_ peer.ID, dir network.Direction) {
	r.streamAllowed.WithLabelValues(getDirection(dir)).Inc()
}

func (r rcmgrMetrics) BlockStream(_ peer.ID, dir network.Direction) {
	r.streamBlocked.WithLabelValues(getDirection(dir)).Inc()
}

func (r rcmgrMetrics) AllowPeer(_ peer.ID) {
	r.peerAllowed.Inc()
}

func (r rcmgrMetrics) BlockPeer(_ peer.ID) {
	r.peerBlocked.Inc()
}

func (r rcmgrMetrics) AllowProtocol(proto protocol.ID) {
	r.protocolAllowed.WithLabelValues(string(proto)).Inc()
}

func (r rcmgrMetrics) BlockProtocol(proto protocol.ID) {
	r.protocolBlocked.WithLabelValues(string(proto)).Inc()
}

func (r rcmgrMetrics) BlockProtocolPeer(proto protocol.ID, _ peer.ID) {
	r.protocolPeerBlocked.WithLabelValues(string(proto)).Inc()
}

func (r rcmgrMetrics) AllowService(svc string) {
	r.serviceAllowed.WithLabelValues(svc).Inc()
}

func (r rcmgrMetrics) BlockService(svc string) {
	r.serviceBlocked.WithLabelValues(svc).Inc()
}

func (r rcmgrMetrics) BlockServicePeer(svc string, _ peer.ID) {
	r.servicePeerBlocked.WithLabelValues(svc).Inc()
}

func (r rcmgrMetrics) AllowMemory(_ int) {
	r.memoryAllowed.Inc()
}

func (r rcmgrMetrics) BlockMemory(_ int) {
	r.memoryBlocked.Inc()
}
