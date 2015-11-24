package ole

import (
	"unicode/utf16"
	"unsafe"
)

// BytePtrToString converts byte pointer to a Go string.
func BytePtrToString(p *byte) string {
	a := (*[10000]uint8)(unsafe.Pointer(p))
	i := 0
	for a[i] != 0 {
		i++
	}
	return string(a[:i])
}

// UTF16PtrToString is alias for LpOleStrToString.
//
// Kept for compatibility reasons.
func UTF16PtrToString(p *uint16) string {
	return LpOleStrToString(p)
}

// LpOleStrToString converts COM Unicode to Go string.
func LpOleStrToString(p *uint16) string {
	if p == nil {
		return ""
	}

	length := lpOleStrLen(p)
	a := make([]uint16, length)

	ptr := unsafe.Pointer(p)

	for i := 0; i < int(length); i++ {
		a[i] = *(*uint16)(ptr)
		ptr = unsafe.Pointer(uintptr(ptr) + 2)
	}

	return string(utf16.Decode(a))
}

// BstrToString converts COM binary string to Go string.
func BstrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	length := SysStringLen((*int16)(unsafe.Pointer(p)))
	a := make([]uint16, length)

	ptr := unsafe.Pointer(p)

	for i := 0; i < int(length); i++ {
		a[i] = *(*uint16)(ptr)
		ptr = unsafe.Pointer(uintptr(ptr) + 2)
	}
	return string(utf16.Decode(a))
}

// lpOleStrLen returns the length of Unicode string.
func lpOleStrLen(p *uint16) (length int64) {
	if p == nil {
		return 0
	}

	ptr := unsafe.Pointer(p)

	for i := 0; ; i++ {
		if 0 == *(*uint16)(ptr) {
			length = int64(i)
			break
		}
		ptr = unsafe.Pointer(uintptr(ptr) + 2)
	}
	return
}

// convertHresultToError converts syscall to error, if call is unsuccessful.
func convertHresultToError(hr uintptr, r2 uintptr, ignore error) (err error) {
	if hr != 0 {
		err = NewError(hr)
	}
	return
}
