package kubo

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	peer "github.com/libp2p/go-libp2p/core/peer"

	pinclient "github.com/ipfs/boxo/pinning/remote/client"
	cid "github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	logging "github.com/ipfs/go-log/v2"

	config "github.com/ipfs/kubo/config"
	"github.com/ipfs/kubo/core"
)

// mfslog is the logger for remote mfs pinning.
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

var daemonConfigPollInterval = time.Minute / 2

func init() {
	// this environment variable is solely for testing, use at your own risk
	if pollDurStr := os.Getenv("MFS_PIN_POLL_INTERVAL"); pollDurStr != "" {
		d, err := time.ParseDuration(pollDurStr)
		if err != nil {
			mfslog.Error("error parsing MFS_PIN_POLL_INTERVAL, using default:", err)
			return
		}
		daemonConfigPollInterval = d
	}
}

const defaultRepinInterval = 5 * time.Minute

type pinMFSContext interface {
	Context() context.Context
	GetConfig() (*config.Config, error)
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

func startPinMFS(cctx pinMFSContext, configPollInterval time.Duration, node pinMFSNode) {
	go pinMFSOnChange(cctx, configPollInterval, node)
}

func pinMFSOnChange(cctx pinMFSContext, configPollInterval time.Duration, node pinMFSNode) {
	tmo := time.NewTimer(configPollInterval)
	defer tmo.Stop()

	lastPins := map[string]lastPin{}
	for {
		// polling sleep
		select {
		case <-cctx.Context().Done():
			return
		case <-tmo.C:
			// reread the config, which may have changed in the meantime
			cfg, err := cctx.GetConfig()
			if err != nil {
				mfslog.Errorf("pinning reading config (%v)", err)
				continue
			}
			mfslog.Debugf("pinning loop is awake, %d remote services", len(cfg.Pinning.RemoteServices))

			// pin to all remote services in parallel
			pinAllMFS(cctx.Context(), node, cfg, lastPins)
		}
		// pinAllMFS may take long. Reset interval only when we are done doing it
		// so that we are not pinning constantly.
		tmo.Reset(configPollInterval)
	}
}

// pinAllMFS pins on all remote services in parallel to overcome DoS attacks.
func pinAllMFS(ctx context.Context, node pinMFSNode, cfg *config.Config, lastPins map[string]lastPin) {
	ch := make(chan lastPin)
	var started int

	// Bail out to mitigate issue below when not needing to do anything.
	if len(cfg.Pinning.RemoteServices) == 0 {
		return
	}

	// get the most recent MFS root cid.
	// Warning! This can be super expensive.
	// See https://github.com/ipfs/boxo/pull/751
	// and https://github.com/ipfs/kubo/issues/8694
	// Reading an MFS-directory nodes can take minutes due to
	// ever growing cache being synced to unixfs.
	rootNode, err := node.RootNode()
	if err != nil {
		mfslog.Errorf("pinning reading MFS root (%v)", err)
		return
	}
	rootCid := rootNode.Cid()

	for svcName, svcConfig := range cfg.Pinning.RemoteServices {
		if ctx.Err() != nil {
			break
		}

		// skip services where MFS is not enabled
		mfslog.Debugf("pinning MFS root considering service %q", svcName)
		if !svcConfig.Policies.MFS.Enable {
			mfslog.Debugf("pinning service %q is not enabled", svcName)
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
				mfslog.Errorf("remote pinning service %q has invalid MFS.RepinInterval (%v)", svcName, err)
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
				continue
			}
		}

		mfslog.Debugf("pinning MFS root %q to %q", rootCid, svcName)
		go func(svcName string, svcConfig config.RemotePinningService) {
			r, err := pinMFS(ctx, node, rootCid, svcName, svcConfig)
			if err != nil {
				mfslog.Errorf("pinning MFS root %q to %q (%v)", rootCid, svcName, err)
			}
			ch <- r
		}(svcName, svcConfig)
		started++
	}

	// Collect results from all started goroutines.
	for i := 0; i < started; i++ {
		if x := <-ch; x.IsValid() {
			lastPins[x.ServiceName] = x
		}
	}
}

func pinMFS(ctx context.Context, node pinMFSNode, cid cid.Cid, svcName string, svcConfig config.RemotePinningService) (lastPin, error) {
	c := pinclient.NewClient(svcConfig.API.Endpoint, svcConfig.API.Key)

	pinName := svcConfig.Policies.MFS.PinName
	if pinName == "" {
		pinName = fmt.Sprintf("policy/%s/mfs", node.Identity().String())
	}

	// check if MFS pin exists (across all possible states) and inspect its CID
	pinStatuses := []pinclient.Status{pinclient.StatusQueued, pinclient.StatusPinning, pinclient.StatusPinned, pinclient.StatusFailed}
	lsPinCh, lsErrCh := c.GoLs(ctx, pinclient.PinOpts.FilterName(pinName), pinclient.PinOpts.FilterStatus(pinStatuses...))
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
	err := <-lsErrCh
	if err != nil {
		return lastPin{}, fmt.Errorf("error while listing remote pins: %v", err)
	}

	if !pinning {
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
			if _, err = c.Replace(ctx, existingRequestID, cid, addOpts...); err != nil {
				return lastPin{}, err
			}
		} else {
			mfslog.Debugf("pinning to %q: creating a new MFS root pin for %q", svcName, cid)
			if _, err = c.Add(ctx, cid, addOpts...); err != nil {
				return lastPin{}, err
			}
		}
	} else {
		mfslog.Debugf("pinning MFS to %q: pin for %q exists since %s, skipping", svcName, cid, pinTime.String())
	}

	return lastPin{
		Time:          pinTime,
		ServiceName:   svcName,
		ServiceConfig: svcConfig,
		CID:           cid,
	}, nil
}
