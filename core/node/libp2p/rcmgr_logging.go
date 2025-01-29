package libp2p

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/filecoin-project/go-clock"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	rcmgr "github.com/libp2p/go-libp2p/p2p/host/resource-manager"
	ma "github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

type loggingResourceManager struct {
	clock       clock.Clock
	logger      *zap.SugaredLogger
	delegate    network.ResourceManager
	logInterval time.Duration

	mut               sync.Mutex
	limitExceededErrs map[string]int
}

type loggingScope struct {
	logger    *zap.SugaredLogger
	delegate  network.ResourceScope
	countErrs func(error)
}

var (
	_ network.ResourceManager    = (*loggingResourceManager)(nil)
	_ rcmgr.ResourceManagerState = (*loggingResourceManager)(nil)
)

func (n *loggingResourceManager) start(ctx context.Context) {
	logInterval := n.logInterval
	if logInterval == 0 {
		logInterval = 10 * time.Second
	}
	ticker := n.clock.Ticker(logInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n.mut.Lock()
				errs := n.limitExceededErrs
				n.limitExceededErrs = make(map[string]int)

				for e, count := range errs {
					n.logger.Warnf("Protected from exceeding resource limits %d times.  libp2p message: %q.", count, e)
				}

				if len(errs) != 0 {
					n.logger.Warnf("Learn more about potential actions to take at: https://github.com/ipfs/kubo/blob/master/docs/libp2p-resource-management.md")
				}

				n.mut.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (n *loggingResourceManager) countErrs(err error) {
	if errors.Is(err, network.ErrResourceLimitExceeded) {
		n.mut.Lock()
		if n.limitExceededErrs == nil {
			n.limitExceededErrs = make(map[string]int)
		}

		// we need to unwrap the error to get the limit scope and the kind of reached limit
		eout := errors.Unwrap(err)
		if eout != nil {
			n.limitExceededErrs[eout.Error()]++
		}

		n.mut.Unlock()
	}
}

func (n *loggingResourceManager) ViewSystem(f func(network.ResourceScope) error) error {
	return n.delegate.ViewSystem(f)
}

func (n *loggingResourceManager) ViewTransient(f func(network.ResourceScope) error) error {
	return n.delegate.ViewTransient(func(s network.ResourceScope) error {
		return f(&loggingScope{logger: n.logger, delegate: s, countErrs: n.countErrs})
	})
}

func (n *loggingResourceManager) ViewService(svc string, f func(network.ServiceScope) error) error {
	return n.delegate.ViewService(svc, func(s network.ServiceScope) error {
		return f(&loggingScope{logger: n.logger, delegate: s, countErrs: n.countErrs})
	})
}

func (n *loggingResourceManager) ViewProtocol(p protocol.ID, f func(network.ProtocolScope) error) error {
	return n.delegate.ViewProtocol(p, func(s network.ProtocolScope) error {
		return f(&loggingScope{logger: n.logger, delegate: s, countErrs: n.countErrs})
	})
}

func (n *loggingResourceManager) ViewPeer(p peer.ID, f func(network.PeerScope) error) error {
	return n.delegate.ViewPeer(p, func(s network.PeerScope) error {
		return f(&loggingScope{logger: n.logger, delegate: s, countErrs: n.countErrs})
	})
}

func (n *loggingResourceManager) OpenConnection(dir network.Direction, usefd bool, remote ma.Multiaddr) (network.ConnManagementScope, error) {
	connMgmtScope, err := n.delegate.OpenConnection(dir, usefd, remote)
	n.countErrs(err)
	return connMgmtScope, err
}

func (n *loggingResourceManager) OpenStream(p peer.ID, dir network.Direction) (network.StreamManagementScope, error) {
	connMgmtScope, err := n.delegate.OpenStream(p, dir)
	n.countErrs(err)
	return connMgmtScope, err
}

func (n *loggingResourceManager) Close() error {
	return n.delegate.Close()
}

func (n *loggingResourceManager) ListServices() []string {
	rapi, ok := n.delegate.(rcmgr.ResourceManagerState)
	if !ok {
		return nil
	}

	return rapi.ListServices()
}

func (n *loggingResourceManager) ListProtocols() []protocol.ID {
	rapi, ok := n.delegate.(rcmgr.ResourceManagerState)
	if !ok {
		return nil
	}

	return rapi.ListProtocols()
}

func (n *loggingResourceManager) ListPeers() []peer.ID {
	rapi, ok := n.delegate.(rcmgr.ResourceManagerState)
	if !ok {
		return nil
	}

	return rapi.ListPeers()
}

func (n *loggingResourceManager) Stat() rcmgr.ResourceManagerStat {
	rapi, ok := n.delegate.(rcmgr.ResourceManagerState)
	if !ok {
		return rcmgr.ResourceManagerStat{}
	}

	return rapi.Stat()
}

func (s *loggingScope) ReserveMemory(size int, prio uint8) error {
	err := s.delegate.ReserveMemory(size, prio)
	s.countErrs(err)
	return err
}

func (s *loggingScope) ReleaseMemory(size int) {
	s.delegate.ReleaseMemory(size)
}

func (s *loggingScope) Stat() network.ScopeStat {
	return s.delegate.Stat()
}

func (s *loggingScope) BeginSpan() (network.ResourceScopeSpan, error) {
	return s.delegate.BeginSpan()
}

func (s *loggingScope) Done() {
	s.delegate.(network.ResourceScopeSpan).Done()
}

func (s *loggingScope) Name() string {
	return s.delegate.(network.ServiceScope).Name()
}

func (s *loggingScope) Protocol() protocol.ID {
	return s.delegate.(network.ProtocolScope).Protocol()
}

func (s *loggingScope) Peer() peer.ID {
	return s.delegate.(network.PeerScope).Peer()
}

func (s *loggingScope) PeerScope() network.PeerScope {
	return s.delegate.(network.PeerScope)
}

func (s *loggingScope) SetPeer(p peer.ID) error {
	err := s.delegate.(network.ConnManagementScope).SetPeer(p)
	s.countErrs(err)
	return err
}

func (s *loggingScope) ProtocolScope() network.ProtocolScope {
	return s.delegate.(network.ProtocolScope)
}

func (s *loggingScope) SetProtocol(proto protocol.ID) error {
	err := s.delegate.(network.StreamManagementScope).SetProtocol(proto)
	s.countErrs(err)
	return err
}

func (s *loggingScope) ServiceScope() network.ServiceScope {
	return s.delegate.(network.ServiceScope)
}

func (s *loggingScope) SetService(srv string) error {
	err := s.delegate.(network.StreamManagementScope).SetService(srv)
	s.countErrs(err)
	return err
}

func (s *loggingScope) Limit() rcmgr.Limit {
	return s.delegate.(rcmgr.ResourceScopeLimiter).Limit()
}

func (s *loggingScope) SetLimit(limit rcmgr.Limit) {
	s.delegate.(rcmgr.ResourceScopeLimiter).SetLimit(limit)
}
