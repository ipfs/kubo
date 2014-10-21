package keytransform_test

import (
	"bytes"
	"sort"
	"testing"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	kt "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/keytransform"
	. "launchpad.net/gocheck"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type DSSuite struct{}

var _ = Suite(&DSSuite{})

func (ks *DSSuite) TestBasic(c *C) {

	pair := &kt.Pair{
		Convert: func(k ds.Key) ds.Key {
			return ds.NewKey("/abc").Child(k)
		},
		Invert: func(k ds.Key) ds.Key {
			// remove abc prefix
			l := k.List()
			if l[0] != "abc" {
				panic("key does not have prefix. convert failed?")
			}
			return ds.KeyWithNamespaces(l[1:])
		},
	}

	mpds := ds.NewMapDatastore()
	ktds := kt.Wrap(mpds, pair)

	keys := strsToKeys([]string{
		"foo",
		"foo/bar",
		"foo/bar/baz",
		"foo/barb",
		"foo/bar/bazb",
		"foo/bar/baz/barb",
	})

	for _, k := range keys {
		err := ktds.Put(k, []byte(k.String()))
		c.Check(err, Equals, nil)
	}

	for _, k := range keys {
		v1, err := ktds.Get(k)
		c.Check(err, Equals, nil)
		c.Check(bytes.Equal(v1.([]byte), []byte(k.String())), Equals, true)

		v2, err := mpds.Get(ds.NewKey("abc").Child(k))
		c.Check(err, Equals, nil)
		c.Check(bytes.Equal(v2.([]byte), []byte(k.String())), Equals, true)
	}

	listA, errA := mpds.KeyList()
	listB, errB := ktds.KeyList()
	c.Check(errA, Equals, nil)
	c.Check(errB, Equals, nil)
	c.Check(len(listA), Equals, len(listB))

	// sort them cause yeah.
	sort.Sort(ds.KeySlice(listA))
	sort.Sort(ds.KeySlice(listB))

	for i, kA := range listA {
		kB := listB[i]
		c.Check(pair.Invert(kA), Equals, kB)
		c.Check(kA, Equals, pair.Convert(kB))
	}
}

func strsToKeys(strs []string) []ds.Key {
	keys := make([]ds.Key, len(strs))
	for i, s := range strs {
		keys[i] = ds.NewKey(s)
	}
	return keys
}
