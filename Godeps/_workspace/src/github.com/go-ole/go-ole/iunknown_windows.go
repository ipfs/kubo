// +build windows

package ole

import (
	"reflect"
	"syscall"
	"unsafe"
)

func reflectQueryInterface(self interface{}, method uintptr, interfaceID *GUID, obj interface{}) (err error) {
	hr, _, _ := syscall.Syscall(
		method,
		3,
		reflect.ValueOf(self).UnsafeAddr(),
		uintptr(unsafe.Pointer(interfaceID)),
		reflect.ValueOf(obj).UnsafeAddr())
	if hr != 0 {
		err = NewError(hr)
	}
	return
}

func queryInterface(unk *IUnknown, iid *GUID) (disp *IDispatch, err error) {
	hr, _, _ := syscall.Syscall(
		unk.VTable().QueryInterface,
		3,
		uintptr(unsafe.Pointer(unk)),
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&disp)))
	if hr != 0 {
		err = NewError(hr)
	}
	return
}

func addRef(unk *IUnknown) int32 {
	ret, _, _ := syscall.Syscall(
		unk.VTable().AddRef,
		1,
		uintptr(unsafe.Pointer(unk)),
		0,
		0)
	return int32(ret)
}

func release(unk *IUnknown) int32 {
	ret, _, _ := syscall.Syscall(
		unk.VTable().Release,
		1,
		uintptr(unsafe.Pointer(unk)),
		0,
		0)
	return int32(ret)
}
