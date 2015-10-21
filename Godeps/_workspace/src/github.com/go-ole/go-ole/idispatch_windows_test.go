// +build windows

package ole

import (
	"reflect"
	"testing"
)

func TestIDispatch(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Error(r)
		}
	}()

	var err error

	err = CoInitialize(0)
	if err != nil {
		t.Fatal(err)
	}

	defer CoUninitialize()

	var unknown *IUnknown
	var dispatch *IDispatch

	// oleutil.CreateObject()
	unknown, err = CreateInstance(CLSID_COMEchoTestObject, IID_IUnknown)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer unknown.Release()

	dispatch, err = unknown.QueryInterface(IID_ICOMEchoTestObject)
	if err != nil {
		t.Fatal(err)
		return
	}
	defer dispatch.Release()

	echoValue := func(method string, value interface{}) (interface{}, bool) {
		var dispid []int32
		var err error

		dispid, err = dispatch.GetIDsOfName([]string{method})
		if err != nil {
			t.Fatal(err)
			return nil, false
		}

		result, err := dispatch.Invoke(dispid[0], DISPATCH_METHOD, value)
		if err != nil {
			t.Fatal(err)
			return nil, false
		}

		return result.Value(), true
	}

	methods := map[string]interface{}{
		"EchoInt8":    int8(1),
		"EchoInt16":   int16(1),
		"EchoInt32":   int32(1),
		"EchoInt64":   int64(1),
		"EchoUInt8":   uint8(1),
		"EchoUInt16":  uint16(1),
		"EchoUInt32":  uint(1),
		"EchoUInt64":  uint64(1),
		"EchoFloat32": float32(1.2),
		"EchoFloat64": float64(1.2),
		"EchoString":  "Test String"}

	for method, expected := range methods {
		if actual, passed := echoValue(method, expected); passed {
			if !reflect.DeepEqual(expected, actual) {
				t.Errorf("%s() expected %v did not match %v", method, expected, actual)
			}
		}
	}
}
