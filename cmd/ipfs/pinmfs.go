package main

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"

	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log"
	pinclient "github.com/ipfs/go-pinning-service-http-client"

	config "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/core"
)

// mfslog is the logger for remote mfs pinning
var mfslog = logging.Logger("remotepinning/mfs")

type lastPin struct {
	Time          time.Time
	ServiceName   string
	ServiceConfig config.RemotePinningService
	CID           cid.Cid
}

func (x lastPin) IsValid() bool {
	return x != lastPin{}
}

const daemonConfigPollInterval = time.Minute / 2
const defaultRepinInterval = 5 * time.Minute

type pinMFSContext interface {
	Context() context.Context
	GetConfigNoCache() (*config.Config, error)
}

type pinMFSNode interface {
	RootNode() (ipld.Node, error)
	Identity() peer.ID
	PeerHost() host.Host
}

type ipfsPinMFSNode struct {
	node *core.IpfsNode
}

func (x *ipfsPinMFSNode) RootNode() (ipld.Node, error) {
	return x.node.FilesRoot.GetDirectory().GetNode()
}

func (x *ipfsPinMFSNode) Identity() peer.ID {
	return x.node.Identity
}

func (x *ipfsPinMFSNode) PeerHost() host.Host {
	return x.node.PeerHost
}

func startPinMFS(configPollInterval time.Duration, cctx pinMFSContext, node pinMFSNode) {
	errCh := make(chan error)
	go pinMFSOnChange(configPollInterval, cctx, node, errCh)
	go func() {
		for {
			select {
			case err, isOpen := <-errCh:
				if !isOpen {
					return
				}
				mfslog.Errorf("%v", err)
			case <-cctx.Context().Done():
				return
			}
		}
	}()
}

func pinMFSOnChange(configPollInterval time.Duration, cctx pinMFSContext, node pinMFSNode, errCh chan<- error) {
	defer close(errCh)

	var tmo *time.Timer
	defer func() {
		if tmo != nil {
			tmo.Stop()
		}
	}()

	lastPins := map[string]lastPin{}
	for {
		// polling sleep
		if tmo == nil {
			tmo = time.NewTimer(configPollInterval)
		} else {
			tmo.Reset(configPollInterval)
		}
		select {
		case <-cctx.Context().Done():
			return
		case <-tmo.C:
		}

		// reread the config, which may have changed in the meantime
		cfg, err := cctx.GetConfigNoCache()
		if err != nil {
			select {
			case errCh <- fmt.Errorf("pinning reading config (%v)", err):
			case <-cctx.Context().Done():
				return
			}
			continue
		}
		mfslog.Debugf("pinning loop is awake, %d remote services", len(cfg.Pinning.RemoteServices))

		// get the most recent MFS root cid
		rootNode, err := node.RootNode()
		if err != nil {
			select {
			case errCh <- fmt.Errorf("pinning reading MFS root (%v)", err):
			case <-cctx.Context().Done():
				return
			}
			continue
		}
		rootCid := rootNode.Cid()

		// pin to all remote services in parallel
		pinAllMFS(cctx.Context(), node, cfg, rootCid, lastPins, errCh)
	}
}

// pinAllMFS pins on all remote services in parallel to overcome DoS attacks.
func pinAllMFS(ctx context.Context, node pinMFSNode, cfg *config.Config, rootCid cid.Cid, lastPins map[string]lastPin, errCh chan<- error) {
	ch := make(chan lastPin, len(cfg.Pinning.RemoteServices))
	for svcName_, svcConfig_ := range cfg.Pinning.RemoteServices {
		// skip services where MFS is not enabled
		svcName, svcConfig := svcName_, svcConfig_
		mfslog.Debugf("pinning MFS root considering service %q", svcName)
		if !svcConfig.Policies.MFS.Enable {
			mfslog.Debugf("pinning service %q is not enabled", svcName)
			ch <- lastPin{}
			continue
		}
		// read mfs pin interval for this service
		var repinInterval time.Duration
		if svcConfig.Policies.MFS.RepinInterval == "" {
			repinInterval = defaultRepinInterval
		} else {
			var err error
			repinInterval, err = time.ParseDuration(svcConfig.Policies.MFS.RepinInterval)
			if err != nil {
				select {
				case errCh <- fmt.Errorf("remote pinning service %q has invalid MFS.RepinInterval (%v)", svcName, err):
				case <-ctx.Done():
				}
				ch <- lastPin{}
				continue
			}
		}

		// do nothing, if MFS has not changed since last pin on the exact same service or waiting for MFS.RepinInterval
		if last, ok := lastPins[svcName]; ok {
			if last.ServiceConfig == svcConfig && (last.CID == rootCid || time.Since(last.Time) < repinInterval) {
				if last.CID == rootCid {
					mfslog.Debugf("pinning MFS root to %q: pin for %q exists since %s, skipping", svcName, rootCid, last.Time.String())
				} else {
					mfslog.Debugf("pinning MFS root to %q: skipped due to MFS.RepinInterval=%s (remaining: %s)", svcName, repinInterval.String(), (repinInterval - time.Since(last.Time)).String())
				}
				ch <- lastPin{}
				continue
			}
		}

		mfslog.Debugf("pinning MFS root %q to %q", rootCid, svcName)
		go func() {
			if r, err := pinMFS(ctx, node, rootCid, svcName, svcConfig); err != nil {
				select {
				case errCh <- fmt.Errorf("pinning MFS root %q to %q (%v)", rootCid, svcName, err):
				case <-ctx.Done():
				}
				ch <- lastPin{}
			} else {
				ch <- r
			}
		}()
	}
	for i := 0; i < len(cfg.Pinning.RemoteServices); i++ {
		if x := <-ch; x.IsValid() {
			lastPins[x.ServiceName] = x
		}
	}
}

func pinMFS(
	ctx context.Context,
	node pinMFSNode,
	cid cid.Cid,
	svcName string,
	svcConfig config.RemotePinningService,
) (lastPin, error) {
	c := pinclient.NewClient(svcConfig.API.Endpoint, svcConfig.API.Key)

	pinName := svcConfig.Policies.MFS.PinName
	if pinName == "" {
		pinName = fmt.Sprintf("policy/%s/mfs", node.Identity().String())
	}

	// check if MFS pin exists (across all possible states) and inspect its CID
	pinStatuses := []pinclient.Status{pinclient.StatusQueued, pinclient.StatusPinning, pinclient.StatusPinned, pinclient.StatusFailed}
	lsPinCh, lsErrCh := c.Ls(ctx, pinclient.PinOpts.FilterName(pinName), pinclient.PinOpts.FilterStatus(pinStatuses...))
	existingRequestID := "" // is there any pre-existing MFS pin with pinName (for any CID)?
	pinning := false        // is CID for current MFS already being pinned?
	pinTime := time.Now().UTC()
	pinStatusMsg := "pinning to %q: received pre-existing %q status for %q (requestid=%q)"
	for ps := range lsPinCh {
		existingRequestID = ps.GetRequestId()
		if ps.GetPin().GetCid() == cid && ps.GetStatus() == pinclient.StatusFailed {
			mfslog.Errorf(pinStatusMsg, svcName, pinclient.StatusFailed, cid, existingRequestID)
		} else {
			mfslog.Debugf(pinStatusMsg, svcName, ps.GetStatus(), ps.GetPin().GetCid(), existingRequestID)
		}
		if ps.GetPin().GetCid() == cid && ps.GetStatus() != pinclient.StatusFailed {
			pinning = true
			pinTime = ps.GetCreated().UTC()
			break
		}
	}
	for range lsPinCh { // in case the prior loop exits early
	}
	if err := <-lsErrCh; err != nil {
		return lastPin{}, fmt.Errorf("error while listing remote pins: %v", err)
	}

	// CID of the current MFS root is already being pinned, nothing to do
	if pinning {
		mfslog.Debugf("pinning MFS to %q: pin for %q exists since %s, skipping", svcName, cid, pinTime.String())
		return lastPin{Time: pinTime, ServiceName: svcName, ServiceConfig: svcConfig, CID: cid}, nil
	}

	// Prepare Pin.name
	addOpts := []pinclient.AddOption{pinclient.PinOpts.WithName(pinName)}

	// Prepare Pin.origins
	// Add own multiaddrs to the 'origins' array, so Pinning Service can
	// use that as a hint and connect back to us (if possible)
	if node.PeerHost() != nil {
		addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(node.PeerHost()))
		if err != nil {
			return lastPin{}, err
		}
		addOpts = append(addOpts, pinclient.PinOpts.WithOrigins(addrs...))
	}

	// Create or replace pin for MFS root
	if existingRequestID != "" {
		mfslog.Debugf("pinning to %q: replacing existing MFS root pin with %q", svcName, cid)
		_, err := c.Replace(ctx, existingRequestID, cid, addOpts...)
		if err != nil {
			return lastPin{}, err
		}
	} else {
		mfslog.Debugf("pinning to %q: creating a new MFS root pin for %q", svcName, cid)
		_, err := c.Add(ctx, cid, addOpts...)
		if err != nil {
			return lastPin{}, err
		}
	}
	return lastPin{Time: pinTime, ServiceName: svcName, ServiceConfig: svcConfig, CID: cid}, nil
}
