package missinggo

import (
	"reflect"

	"gx/ipfs/QmYWL7Pyx6QHHryhLq96wR6CWidApH2D2nbXeTJbAmusH9/iter"
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
