package oleutil

import ole "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/go-ole/go-ole"

// ClassIDFrom retrieves class ID whether given is program ID or application string.
func ClassIDFrom(programID string) (classID *ole.GUID, err error) {
	classID, err = ole.CLSIDFromProgID(programID)
	if err != nil {
		classID, err = ole.CLSIDFromString(programID)
		if err != nil {
			return
		}
	}
	return
}

// CreateObject creates object from programID based on interface type.
//
// Only supports IUnknown.
//
// Program ID can be either program ID or application string.
func CreateObject(programID string) (unknown *ole.IUnknown, err error) {
	classID, err := ClassIDFrom(programID)
	if err != nil {
		return
	}

	unknown, err = ole.CreateInstance(classID, ole.IID_IUnknown)
	if err != nil {
		return
	}

	return
}

// GetActiveObject retrieves active object for program ID and interface ID based
// on interface type.
//
// Only supports IUnknown.
//
// Program ID can be either program ID or application string.
func GetActiveObject(programID string) (unknown *ole.IUnknown, err error) {
	classID, err := ClassIDFrom(programID)
	if err != nil {
		return
	}

	unknown, err = ole.GetActiveObject(classID, ole.IID_IUnknown)
	if err != nil {
		return
	}

	return
}

// CallMethod calls method on IDispatch with parameters.
func CallMethod(disp *ole.IDispatch, name string, params ...interface{}) (result *ole.VARIANT, err error) {
	var dispid []int32
	dispid, err = disp.GetIDsOfName([]string{name})
	if err != nil {
		return
	}

	if len(params) < 1 {
		result, err = disp.Invoke(dispid[0], ole.DISPATCH_METHOD)
	} else {
		result, err = disp.Invoke(dispid[0], ole.DISPATCH_METHOD, params...)
	}

	return
}

// MustCallMethod calls method on IDispatch with parameters or panics.
func MustCallMethod(disp *ole.IDispatch, name string, params ...interface{}) (result *ole.VARIANT) {
	r, err := CallMethod(disp, name, params...)
	if err != nil {
		panic(err.Error())
	}
	return r
}

// GetProperty retrieves property from IDispatch.
func GetProperty(disp *ole.IDispatch, name string, params ...interface{}) (result *ole.VARIANT, err error) {
	var dispid []int32
	dispid, err = disp.GetIDsOfName([]string{name})
	if err != nil {
		return
	}

	if len(params) < 1 {
		result, err = disp.Invoke(dispid[0], ole.DISPATCH_PROPERTYGET)
	} else {
		result, err = disp.Invoke(dispid[0], ole.DISPATCH_PROPERTYGET, params...)
	}

	return
}

// MustGetProperty retrieves property from IDispatch or panics.
func MustGetProperty(disp *ole.IDispatch, name string, params ...interface{}) (result *ole.VARIANT) {
	r, err := GetProperty(disp, name, params...)
	if err != nil {
		panic(err.Error())
	}
	return r
}

// PutProperty mutates property.
func PutProperty(disp *ole.IDispatch, name string, params ...interface{}) (result *ole.VARIANT, err error) {
	var dispid []int32
	dispid, err = disp.GetIDsOfName([]string{name})
	if err != nil {
		return
	}

	if len(params) < 1 {
		result, err = disp.Invoke(dispid[0], ole.DISPATCH_PROPERTYPUT)
	} else {
		result, err = disp.Invoke(dispid[0], ole.DISPATCH_PROPERTYPUT, params...)
	}

	return
}

// MustPutProperty mutates property or panics.
func MustPutProperty(disp *ole.IDispatch, name string, params ...interface{}) (result *ole.VARIANT) {
	r, err := PutProperty(disp, name, params...)
	if err != nil {
		panic(err.Error())
	}
	return r
}
