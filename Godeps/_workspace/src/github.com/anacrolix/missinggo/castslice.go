package missinggo

import (
	"reflect"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/bradfitz/iter"
)

func ConvertToSliceOfEmptyInterface(slice interface{}) (ret []interface{}) {
	v := reflect.ValueOf(slice)
	l := v.Len()
	ret = make([]interface{}, 0, l)
	for i := range iter.N(v.Len()) {
		ret = append(ret, v.Index(i).Interface())
	}
	return
}
