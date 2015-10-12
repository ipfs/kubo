package itertools

import "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/anacrolix/missinggo"

type Iterator interface {
	Next() bool
	Value() interface{}
}

type sliceIterator struct {
	slice []interface{}
	value interface{}
	ok    bool
}

func (me *sliceIterator) Next() bool {
	if len(me.slice) == 0 {
		return false
	}
	me.value = me.slice[0]
	me.slice = me.slice[1:]
	me.ok = true
	return true
}

func (me *sliceIterator) Value() interface{} {
	if !me.ok {
		panic("no value; call Next")
	}
	return me.value
}

func SliceIterator(a []interface{}) Iterator {
	return &sliceIterator{
		slice: a,
	}
}

func StringIterator(a string) Iterator {
	return SliceIterator(missinggo.ConvertToSliceOfEmptyInterface(a))
}
