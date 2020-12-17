package main

import (
	"context"
	_ "expvar"
	"fmt"
	_ "net/http/pprof"
	"time"

	cid "github.com/ipfs/go-cid"
	config "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/core"
	ipld "github.com/ipfs/go-ipld-format"
	pinclient "github.com/ipfs/go-pinning-service-http-client"
	"github.com/libp2p/go-libp2p-core/host"
	peer "github.com/libp2p/go-libp2p-core/peer"
)

type lastPin struct {
	Time          time.Time
	ServiceName   string
	ServiceConfig config.RemotePinningService
	CID           cid.Cid
}

const configPollInterval = time.Minute / 2

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
	return x.PeerHost()
}

func pinMFSOnChange(cctx pinMFSContext, node pinMFSNode, errCh chan<- error) error {
	go func() {
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
				log.Errorf("pinning reading config (%v)", err)
				select {
				case errCh <- err:
				case <-cctx.Context().Done():
					return //COV
				}
				continue
			}
			log.Infof("pinning loop is awake, %d remote services", len(cfg.Pinning.RemoteServices))

			// get the most recent MFS root cid
			rootNode, err := node.RootNode()
			if err != nil {
				log.Errorf("pinning reading mfs root (%v)", err)
				select {
				case errCh <- err:
				case <-cctx.Context().Done():
					return //COV
				}
				continue
			}
			rootCid := rootNode.Cid()

			// pin on all remote services in parallel to prevent DoS attacks
			ch := make(chan lastPin, len(cfg.Pinning.RemoteServices))
			for svcName_, svcConfig_ := range cfg.Pinning.RemoteServices {
				// skip services where MFS is not enabled
				svcName, svcConfig := svcName_, svcConfig_ //COV
				log.Infof("pinning considering service %s for mfs pinning", svcName)
				if !svcConfig.Policies.MFS.Enable {
					log.Infof("pinning service %s is not enabled", svcName)
					continue //COV
				}
				// read mfs pin interval for this service
				repinInterval, err := time.ParseDuration(svcConfig.Policies.MFS.RepinInterval) //COV
				if err != nil {
					log.Errorf("pinning parsing service %s repin interval %q", svcName, svcConfig.Policies.MFS.RepinInterval)
					select {
					case errCh <- fmt.Errorf("remote pinning service %s has invalid mfs pin interval (%v)", svcName, err):
					case <-cctx.Context().Done():
						return //COV
					}
					continue //COV
				}

				// do nothing, if MFS has not changed since last pin on the exact same service
				if last, ok := lastPins[svcName]; ok { //COV
					if last.ServiceConfig == svcConfig && last.CID == rootCid && time.Since(last.Time) < repinInterval {
						log.Infof("pinning mfs was pinned to %s recently, skipping", svcName)
						ch <- lastPin{}
						continue //COV
					}
				}

				log.Infof("pinning mfs root %s to %s", rootCid, svcName) //COV
				go func() {
					if r, err := pinMFS(cctx.Context(), node, rootCid, svcName, svcConfig, errCh); err != nil {
						ch <- lastPin{} //COV
					} else {
						ch <- r //COV
					}
				}()
			}
			for i := 0; i < len(cfg.Pinning.RemoteServices); i++ {
				x := <-ch                   //COV
				lastPins[x.ServiceName] = x //COV
			}
		}
	}()
	return nil
}

func pinMFS(
	ctx context.Context,
	node pinMFSNode,
	cid cid.Cid,
	svcName string,
	svcConfig config.RemotePinningService,
	errCh chan<- error,
) (lastPin, error) {
	c := pinclient.NewClient(svcConfig.API.Endpoint, svcConfig.API.Key)

	pinName := svcConfig.Policies.MFS.PinName
	if pinName == "" {
		pinName = fmt.Sprintf("policy/%s/mfs", node.Identity().String())
	}

	// check if same pin exists
	lsPinCh, lsErrCh := c.Ls(ctx, pinclient.PinOpts.FilterCIDs(cid), pinclient.PinOpts.FilterName(pinName))
	pinFound := false
	for ps := range lsPinCh {
		if ps.GetPin().GetCid() == cid {
			pinFound = true
		}
	}
	if err := <-lsErrCh; err != nil {
		err = fmt.Errorf("error while listing remote pins: %v", err)
		select {
		case errCh <- err:
		case <-ctx.Done():
		}
		return lastPin{}, err
	}
	if pinFound {
		return lastPin{}, nil
	}

	// Prepare Pin.name
	addOpts := []pinclient.AddOption{pinclient.PinOpts.WithName(pinName)}

	// Prepare Pin.origins
	// Add own multiaddrs to the 'origins' array, so Pinning Service can
	// use that as a hint and connect back to us (if possible)
	if node.PeerHost() != nil {
		addrs, err := peer.AddrInfoToP2pAddrs(host.InfoFromHost(node.PeerHost()))
		if err != nil {
			select {
			case errCh <- err:
			case <-ctx.Done():
			}
			return lastPin{}, err
		}
		addOpts = append(addOpts, pinclient.PinOpts.WithOrigins(addrs...))
	}

	// Execute remote pin request
	_, err := c.Add(ctx, cid, addOpts...)
	if err != nil {
		select {
		case errCh <- err:
		case <-ctx.Done():
		}
		return lastPin{}, err
	}
	return lastPin{Time: time.Now(), ServiceName: svcName, ServiceConfig: svcConfig, CID: cid}, nil
}
