package namespace

import (
	"fmt"
	"strings"

	ds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	ktds "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/keytransform"
)

// PrefixTransform constructs a KeyTransform with a pair of functions that
// add or remove the given prefix key.
//
// Warning: will panic if prefix not found when it should be there. This is
// to avoid insidious data inconsistency errors.
func PrefixTransform(prefix ds.Key) ktds.KeyTransform {
	return &ktds.Pair{

		// Convert adds the prefix
		Convert: func(k ds.Key) ds.Key {
			return prefix.Child(k)
		},

		// Invert removes the prefix. panics if prefix not found.
		Invert: func(k ds.Key) ds.Key {
			if !prefix.IsAncestorOf(k) {
				fmt.Errorf("Expected prefix (%s) in key (%s)", prefix, k)
				panic("expected prefix not found")
			}

			s := strings.TrimPrefix(k.String(), prefix.String())
			return ds.NewKey(s)
		},
	}
}

// Wrap wraps a given datastore with a key-prefix.
func Wrap(child ds.Datastore, prefix ds.Key) ktds.Datastore {
	if child == nil {
		panic("child (ds.Datastore) is nil")
	}

	return ktds.Wrap(child, PrefixTransform(prefix))
}
