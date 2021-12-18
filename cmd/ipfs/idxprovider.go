package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"io"

	gsimpl "github.com/ipfs/go-graphsync/impl"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/multiformats/go-multihash"

	datatransferImpl "github.com/filecoin-project/go-data-transfer/impl"
	dtnetwork "github.com/filecoin-project/go-data-transfer/network"
	gstransport "github.com/filecoin-project/go-data-transfer/transport/graphsync"
	"github.com/filecoin-project/go-legs"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	leveldb "github.com/ipfs/go-ds-leveldb"
	"github.com/ipfs/go-graphsync"
	gsnetwork "github.com/ipfs/go-graphsync/network"
	logging "github.com/ipfs/go-log"
	"github.com/ipld/go-ipld-prime"
	"github.com/ipld/go-ipld-prime/codec/dagjson"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/ipld/go-ipld-prime/node/basicnode"

	config "github.com/ipfs/go-ipfs/config"
	"github.com/ipfs/go-ipfs/core"

	"github.com/filecoin-project/go-legs/dtsync"
	stiapi "github.com/filecoin-project/storetheindex/api/v0"
	httpc "github.com/filecoin-project/storetheindex/api/v0/ingest/client/http"
	"github.com/filecoin-project/storetheindex/api/v0/ingest/schema"
	bstore "github.com/ipfs/go-ipfs-blockstore"
)

var providerlog = logging.Logger("idxProvider")

type idxProviderContext interface {
	Context() context.Context
	GetConfigNoCache() (*config.Config, error)
}

type idxProviderNode interface {
	PeerHost() host.Host
	BlockStore() bstore.GCBlockstore
	GraphExchange() graphsync.GraphExchange
	Addrs() []string
	DataStore() datastore.Batching
}

type ipfsIdxProviderNode struct {
	node *core.IpfsNode
}

func (x *ipfsIdxProviderNode) PeerHost() host.Host {
	return x.node.PeerHost
}

func (x *ipfsIdxProviderNode) BlockStore() bstore.GCBlockstore {
	return x.node.Blockstore
}

func (x *ipfsIdxProviderNode) GraphExchange() graphsync.GraphExchange {
	return x.node.GraphExchange
}

func (x *ipfsIdxProviderNode) DataStore() datastore.Batching {
	return x.node.Repo.Datastore()
}

func (x *ipfsIdxProviderNode) Addrs() []string {
	addrs := make([]string, len(x.node.PeerHost.Addrs()))
	for i, m := range x.node.PeerHost.Addrs() {
		addrs[i] = m.String()
	}
	return addrs
}

const (
	latestAdvKey    = "sync/adv/"
	linkedChunkSize = 100
	pubSubTopic     = "/indexer/ingest/mainnet"
	maxCidsPerAdv   = 1000
	datastorePath   = "idxproviderdatastore"
)

var dataStore datastore.Batching
var lsys ipld.LinkSystem
var lp legs.Publisher

func setupIdxProvider(cctx idxProviderContext, node idxProviderNode, indexerNode string) error {
	providerlog.Debugf("starting idx provider")

	var err error

	if dataStore == nil {
		dataStorePath, err := config.Path("", datastorePath)
		if err != nil {
			providerlog.Errorf("error getting datastore path: %v", err)
			return err
		}
		err = checkWritable(dataStorePath)
		if err != nil {
			providerlog.Errorf("Error checking if datastorepath is writable: %v", err)
			return err
		}
		dataStore, err = leveldb.NewDatastore(dataStorePath, nil)
		if err != nil {
			providerlog.Errorf("Error creating new leveldb datastore: %v", err)
			return err
		}
		// TODO why can't I use this datastore instead of creating my own?  When I try I get an panic in migrations code
		// with an empty array being indexed
		// dataStore := node.DataStore()

		providerlog.Debugf("registering with indexer")
		err = registerWithIndexers(cctx, node, indexerNode)
		if err != nil {
			providerlog.Errorf("Error registering with indexer: %v", err)
			return err
		}

		providerlog.Debugf("creating net, transport and datatransfer")

		providerlog.Debugf("creating link system")
		lsys = mkLinkSystem()

		gsNet := gsnetwork.NewFromLibp2pHost(node.PeerHost())
		gs := gsimpl.New(cctx.Context(), gsNet, lsys)

		// TODO currently can't use this version since it has it's own linksystem - could swap that out
		// based on idx provider being enabled, but saving that for a later time
		// gs := node.GraphExchange()

		tp := gstransport.NewTransport(node.PeerHost().ID(), gs)
		dtNet := dtnetwork.NewFromLibp2pHost(node.PeerHost())
		dt, err := datatransferImpl.NewDataTransfer(dataStore, dtNet, tp)
		if err != nil {
			providerlog.Errorf("Error creating data transfer: %v", err)
			return err
		}

		err = dt.Start(context.Background())
		if err != nil {
			providerlog.Errorf("Error starting data transfer: %v", err)
			return err
		}

		providerlog.Debugf("creating publisher")
		lp, err = dtsync.NewPublisherFromExisting(dt, node.PeerHost(), pubSubTopic, lsys)
		if err != nil {
			providerlog.Errorf("Error initializing publisher in engine: %s", err)
			return err
		}
	}
	return err
}

func registerWithIndexers(cctx idxProviderContext, node idxProviderNode, indexerNode string) error {
	providerlog.Debugf("registerWithIndexers")

	client, err := httpc.New(indexerNode)
	if err != nil {
		return err
	}

	peerID := node.PeerHost().ID()
	privKey := node.PeerHost().Peerstore().PrivKey(node.PeerHost().ID())
	providerlog.Debugf("calling storetheindex register")
	return client.Register(cctx.Context(), peerID, privKey, node.Addrs())
}

func advertiseAllCids(ctx context.Context, node idxProviderNode) error {
	providerlog.Debugf("Advertising latest CIDs")

	allCids, err := node.BlockStore().AllKeysChan(ctx)
	if err != nil {
		return err
	}

	var cnt = 0
	var cids []cid.Cid

	for c := range allCids {
		cids = append(cids, c)
		cids[cnt] = c
		cnt++
		if cnt >= maxCidsPerAdv {
			err = doAdvertisement(ctx, node, cids)
			if err != nil {
				providerlog.Errorf("Error advertising: %s", err)
				return err
			}
			// reset array for next window
			cids = cids[:0]
			cnt = 0
		}

	}
	// advertise the leftovers
	err = doAdvertisement(ctx, node, cids)
	if err != nil {
		providerlog.Errorf("Error advertising: %s", err)
	}
	return err
}

func doAdvertisement(ctx context.Context, node idxProviderNode, cids []cid.Cid) error {
	providerlog.Debugf("generating chunks for %v cids", len(cids))
	lnk, contextID, err := generateChunks(ctx, cids)
	if err != nil {
		providerlog.Debug("Error generating chunks")
		return err
	}
	if lnk == nil {
		providerlog.Debug("Nothing to advertise this time")
		return nil
	}

	cidsLnk := lnk.(cidlink.Link)

	providerlog.Debugf("generating bogus metadata")
	// 0x0900 is the bitswap protocol ID
	metadata := stiapi.Metadata{
		ProtocolID: 0x0900,
	}

	peerID := node.PeerHost().ID()
	privKey := node.PeerHost().Peerstore().PrivKey(node.PeerHost().ID())

	// Check for cid.Undef for the previous link. If this is the case, then
	// this means there is a "cid too short" error in IPLD links serialization.
	previousLnk, err := getPreviousAdvertisementLink(ctx)
	if err != nil {
		providerlog.Warnf("Error getting previous adv link, sending nil:  %s", err)
		previousLnk = nil
	}

	providerlog.Debugf("creating new adv")
	adv, err := schema.NewAdvertisement(privKey, previousLnk, cidsLnk,
		contextID, metadata, false, peerID.String(), node.Addrs())
	if err != nil {
		providerlog.Debugf("got error from new advertisement: %v", err)
		return err
	}

	providerlog.Debugf("creating new adv link")
	adLnk, err := schema.AdvertisementLink(lsys, adv)
	if err != nil {
		providerlog.Errorf("Error generating advertisement link: %s", err)
		return err
	}

	c := adLnk.ToCid()

	// Store latest advertisement published from the chain
	providerlog.Infow("Storing advertisement locally", "cid", c.String())
	err = dataStore.Put(ctx, datastore.NewKey(latestAdvKey), c.Bytes())
	if err != nil {
		log.Errorf("Error storing latest advertisement in blockstore: %s", err)
		return err
	}

	providerlog.Debugf("updateRoot to publish pubsub message")
	err = lp.UpdateRoot(ctx, c)
	if err != nil {
		log.Errorf("Error updating root of lp: %s", err)
	}
	return nil
}

func getPreviousAdvertisementLink(ctx context.Context) (schema.Link_Advertisement, error) {
	latestAdvID, err := getLatestAdv(ctx)
	if err != nil {
		log.Errorf("Could not get latest advertisement: %s", err)
	}
	var previousLnk schema.Link_Advertisement

	if latestAdvID == cid.Undef {
		log.Warn("Latest advertisement CID was undefined")
		previousLnk = nil
	} else {
		nb := schema.Type.Link_Advertisement.NewBuilder()
		err = nb.AssignLink(cidlink.Link{Cid: latestAdvID})
		if err != nil {
			log.Errorf("Error generating link from latest advertisement: %s", err)
			return nil, err
		}
		previousLnk = nb.Build().(schema.Link_Advertisement)
	}
	return previousLnk, nil
}

func getLatestAdv(ctx context.Context) (cid.Cid, error) {
	b, err := dataStore.Get(ctx, datastore.NewKey(latestAdvKey))
	if err != nil {
		if err == datastore.ErrNotFound {
			return cid.Undef, nil
		}
		return cid.Undef, err
	}
	_, c, err := cid.CidFromBytes(b)
	return c, err
}

// Checks if an IPLD node is an advertisement or
// an index.
// (We may need additional checks if we extend
// the schema with new types that are traversable)
func isAdvertisement(n ipld.Node) bool {
	indexID, _ := n.LookupByString("Signature")
	return indexID != nil
}

// generateChunks iterates multihashes, bundles them into a chunk (slice), and
// then and stores that chunk and a link to the previous chunk.
func generateChunks(ctx context.Context, cids []cid.Cid) (ipld.Link, []byte, error) {
	providerlog.Debugf("generateChunks")
	providerlog.Debugf("cidCount: %d", len(cids))
	chunkSize := linkedChunkSize
	mhs := make([]multihash.Multihash, 0, chunkSize)

	var chunkLnk ipld.Link
	var totalMhCount, chunkCount int

	for _, c := range cids {
		providerlog.Debugf("cid: %s", c)
		mhs = append(mhs, c.Hash())
		totalMhCount++

		if len(mhs) >= chunkSize {
			var err error
			chunkLnk, _, err = schema.NewLinkedListOfMhs(lsys, mhs, chunkLnk)
			if err != nil {
				providerlog.Debugf("error generating new linked list of mhs: %v", err)
				return nil, nil, err
			}
			chunkCount++
			// NewLinkedListOfMhs makes it own copy, so safe to reuse mhs
			mhs = mhs[:0]
		}
	}

	// Chunk remaining multihashes.
	if len(mhs) != 0 {
		var err error
		chunkLnk, _, err = schema.NewLinkedListOfMhs(lsys, mhs, chunkLnk)
		if err != nil {
			providerlog.Debugf("error generating new linked list of mhs: %v", err)
			return nil, nil, err
		}
		chunkCount++
	}
	contextID, err := generateContextID()
	if err != nil {
		providerlog.Errorf("Error generating contextID: %s", err)
		return nil, nil, err
	}
	providerlog.Infow("Generated linked chunks of multihashes", "totalMhCount", totalMhCount, "chunkCount", chunkCount)
	if totalMhCount == 0 {
		providerlog.Infow("No new CIDs to advertise")
	}

	return chunkLnk, contextID, nil
}

func generateContextID() ([]byte, error) {
	providerlog.Debugf("generating random contextID")
	data := make([]byte, 10)
	if _, err := rand.Read(data); err != nil {
		return nil, err
	}
	h := sha256.New()
	if _, err := h.Write(data); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

// decodeIPLDNode from a reaed
// This is used to get the ipld.Node from a set of raw bytes.
func decodeIPLDNode(r io.Reader) (ipld.Node, error) {
	providerlog.Debugf("decodeIPLDNode")

	nb := basicnode.Prototype.Any.NewBuilder()
	err := dagjson.Decode(nb, r)
	if err != nil {
		return nil, err
	}
	return nb.Build(), nil
}

func mkLinkSystem() ipld.LinkSystem {
	providerlog.Debugf("mkLinkSystem")

	lsys := cidlink.DefaultLinkSystem()
	lsys.StorageReadOpener = func(lctx ipld.LinkContext, lnk ipld.Link) (io.Reader, error) {
		c := lnk.(cidlink.Link).Cid
		providerlog.Debugf("Triggered ReadOpener from engine's linksystem with cid (%s)", c)

		// Get the node from main datastore. If it is in the
		// main datastore it means it is an advertisement.
		val, err := dataStore.Get(lctx.Ctx, datastore.NewKey(c.String()))
		if err != nil && err != datastore.ErrNotFound {
			providerlog.Errorf("Error getting object from datastore in linksystem: %s", err)
			return nil, err
		}
		// If data was retrieved from the datastore, this may be an advertisement.
		if len(val) != 0 {
			// Decode the node to check its type to see if it is an Advertisement.
			n, err := decodeIPLDNode(bytes.NewBuffer(val))
			if err != nil {
				providerlog.Errorf("Could not decode IPLD node for potential advertisement: %s", err)
				return nil, err
			}
			// If this was an advertisement, then return it.
			if isAdvertisement(n) {
				providerlog.Infow("Retrieved advertisement from datastore", "cid", c, "size", len(val))
				return bytes.NewBuffer(val), nil
			}
			providerlog.Infow("Retrieved non-advertisement object from datastore", "cid", c, "size", len(val))
		}

		return bytes.NewBuffer(val), nil
	}
	lsys.StorageWriteOpener = func(lctx ipld.LinkContext) (io.Writer, ipld.BlockWriteCommitter, error) {
		providerlog.Debugf("Triggered WriteOpener from engine's linksystem")
		buf := bytes.NewBuffer(nil)
		return buf, func(lnk ipld.Link) error {
			c := lnk.(cidlink.Link).Cid
			key := datastore.NewKey(c.String())
			val := buf.Bytes()
			return dataStore.Put(lctx.Ctx, key, val)
		}, nil
	}
	return lsys
}
