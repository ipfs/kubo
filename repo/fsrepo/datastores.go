package fsrepo

import (
	"fmt"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/crowdmob/goamz/aws"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/crowdmob/goamz/s3"

	repo "github.com/ipfs/go-ipfs/repo"
	config "github.com/ipfs/go-ipfs/repo/config"
	"github.com/ipfs/go-ipfs/thirdparty/s3-datastore"
)

func openS3Datastore(params config.S3Datastore) (repo.Datastore, error) {
	// TODO support credentials files
	auth, err := aws.EnvAuth()
	if err != nil {
		return nil, err
	}

	region := aws.GetRegion(params.Region)
	if region.Name == "" {
		return nil, fmt.Errorf("unknown AWS region: %q", params.Region)
	}

	if params.Bucket == "" {
		return nil, fmt.Errorf("invalid S3 bucket: %q", params.Bucket)
	}

	client := s3.New(auth, region)
	// There are too many gophermucking s3datastores in my
	// gophermucking source.
	return &s3datastore.S3Datastore{
		Client: client,
		Bucket: params.Bucket,
		ACL:    s3.ACL(params.ACL),
	}, nil
}
