package key

type KeySet interface {
	Add(Key)
	Has(Key) bool
	Remove(Key)
	Keys() []Key
}

type keySet struct {
	keys map[Key]struct{}
}

func NewKeySet() KeySet {
	return &keySet{make(map[Key]struct{})}
}

func (gcs *keySet) Add(k Key) {
	gcs.keys[k] = struct{}{}
}

func (gcs *keySet) Has(k Key) bool {
	_, has := gcs.keys[k]
	return has
}

func (ks *keySet) Keys() []Key {
	var out []Key
	for k, _ := range ks.keys {
		out = append(out, k)
	}
	return out
}

func (ks *keySet) Remove(k Key) {
	delete(ks.keys, k)
}

// TODO: implement disk-backed keyset for working with massive DAGs
