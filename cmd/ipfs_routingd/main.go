package main

import (
	"flag"
	"log"
	"os"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	aws "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/crowdmob/goamz/aws"
	s3 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/crowdmob/goamz/s3"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	syncds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/sync"
	core "github.com/jbenet/go-ipfs/core"
	corehttp "github.com/jbenet/go-ipfs/core/corehttp"
	corerouting "github.com/jbenet/go-ipfs/core/corerouting"
	config "github.com/jbenet/go-ipfs/repo/config"
	fsrepo "github.com/jbenet/go-ipfs/repo/fsrepo"
	s3datastore "github.com/jbenet/go-ipfs/thirdparty/s3-datastore"
	ds2 "github.com/jbenet/go-ipfs/util/datastore2"
)

var (
	host            = flag.String("host", "/ip4/0.0.0.0/tcp/4001", "override the host listening address")
	s3bucket        = flag.String("aws-bucket", "", "S3 bucket for routing datastore")
	s3region        = flag.String("aws-region", aws.USWest2.Name, "S3 region")
	nBitsForKeypair = flag.Int("b", 1024, "number of bits for keypair (if repo is uninitialized)")
)

func main() {
	flag.Parse()
	if *s3bucket == "" {
		log.Fatal("bucket is required")
	}
	if err := run(); err != nil {
		log.Println(err)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repoPath, err := fsrepo.BestKnownPath()
	if err != nil {
		return err
	}

	if !fsrepo.IsInitialized(repoPath) {
		conf, err := config.Init(os.Stdout, *nBitsForKeypair)
		if err != nil {
			return err
		}
		if err := fsrepo.Init(repoPath, conf); err != nil {
			return err
		}
	}
	repo := fsrepo.At(repoPath)
	if err := repo.Open(); err != nil { // owned by node
		return err
	}
	s3, err := makeS3Datastore()
	if err != nil {
		return err
	}
	enhanced, err := enhanceDatastore(s3)
	if err != nil {
		return err
	}
	node, err := core.NewIPFSNode(ctx,
		core.OnlineWithOptions(
			repo,
			corerouting.SupernodeServer(enhanced),
			core.DefaultHostOption),
	)
	if err != nil {
		return err
	}
	defer node.Close()

	opts := []corehttp.ServeOption{}
	return corehttp.ListenAndServe(node, *host, opts...) // TODO rm
}

func makeS3Datastore() (*s3datastore.S3Datastore, error) {

	// FIXME get ENV through flags?

	auth, err := aws.EnvAuth()
	if err != nil {
		return nil, err
	}

	s3c := s3.New(auth, aws.Regions[*s3region])
	b := s3c.Bucket(*s3bucket)
	exists, err := b.Exists("initialized") // TODO lazily instantiate
	if err != nil {
		return nil, err
	}

	if !exists {
		if err := b.PutBucket(s3.PublicRead); err != nil {
			switch e := err.(type) {
			case *s3.Error:
				log.Println(e.Code)
			default:
				return nil, err
			}
		}

		// TODO create the initial value
	}

	return &s3datastore.S3Datastore{
		Bucket: *s3bucket,
		Client: s3c,
	}, nil
}

func enhanceDatastore(d datastore.Datastore) (datastore.ThreadSafeDatastore, error) {
	// TODO cache
	return ds2.CloserWrap(syncds.MutexWrap(d)), nil
}
