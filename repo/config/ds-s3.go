package config

import (
	"encoding/json"
	"fmt"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/crowdmob/goamz/aws"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/crowdmob/goamz/s3"
	"github.com/ipfs/go-ipfs/thirdparty/s3-datastore"
	"github.com/ipfs/go-ipfs/util/datastore2"
)

type s3Datastore struct {
	region aws.Region
	bucket string
	acl    s3.ACL
}

var _ json.Unmarshaler = (*s3Datastore)(nil)

type s3json struct {
	Type   string
	Region string
	Bucket string
	ACL    string `json:",omitempty"`
}

func (s *s3Datastore) UnmarshalJSON(data []byte) error {
	var raw s3json
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	s.region = aws.GetRegion(raw.Region)
	if s.region.Name == "" {
		return fmt.Errorf("unknown AWS region: %q", raw.Region)
	}

	if raw.Bucket == "" {
		return fmt.Errorf("invalid S3 bucket: %q", raw.Bucket)
	}
	s.bucket = raw.Bucket

	// it would be nice to have validation for this, but the S3 does
	// not provide a list
	s.acl = s3.ACL(raw.ACL)

	return nil
}

var _ json.Marshaler = (*s3Datastore)(nil)

func (s s3Datastore) MarshalJSON() ([]byte, error) {
	raw := s3json{
		Type:   "s3",
		Region: s.region.Name,
		Bucket: s.bucket,
		ACL:    string(s.acl),
	}
	return json.Marshal(&raw)
}

var _ DSOpener = s3Datastore{}

func (s s3Datastore) Open(repoPath string) (IPFSDatastore, error) {
	// TODO support credentials files
	auth, err := aws.EnvAuth()
	if err != nil {
		return nil, err
	}

	client := s3.New(auth, s.region)
	// There are too many gophermucking s3datastores in my
	// gophermucking source.
	ds := datastore2.CloserWrap(
		&s3datastore.S3Datastore{
			Client: client,
			Bucket: s.bucket,
			ACL:    s3.ACL(s.acl),
		},
	)
	return ds, nil
}
