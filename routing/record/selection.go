package record

import (
	"errors"

	path "github.com/ipfs/go-ipfs/path"
	key "gx/ipfs/Qmce4Y4zg3sYr7xKM5UueS67vhNni6EeWgCRnb7MbLJMew/go-key"
)

// A SelectorFunc selects the best value for the given key from
// a slice of possible values and returns the index of the chosen one
type SelectorFunc func(key.Key, [][]byte) (int, error)

type Selector map[string]SelectorFunc

func (s Selector) BestRecord(k key.Key, recs [][]byte) (int, error) {
	if len(recs) == 0 {
		return 0, errors.New("no records given!")
	}

	parts := path.SplitList(string(k))
	if len(parts) < 3 {
		log.Infof("Record key does not have selectorfunc: %s", k)
		return 0, errors.New("record key does not have selectorfunc")
	}

	sel, ok := s[parts[1]]
	if !ok {
		log.Infof("Unrecognized key prefix: %s", parts[1])
		return 0, ErrInvalidRecordType
	}

	return sel(k, recs)
}

// PublicKeySelector just selects the first entry.
// All valid public key records will be equivalent.
func PublicKeySelector(k key.Key, vals [][]byte) (int, error) {
	return 0, nil
}
