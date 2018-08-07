//+build windows

package commands

import (
	"fmt"
	"go/build"
	"os"
	"syscall"
	"unsafe"

	tar "gx/ipfs/QmTkC7aeyDyjfdMTCVcG9P485TMJd6foLaLbf11DZ5WrnV/tar-utils"
	windows "gx/ipfs/QmVGjyM9i2msKvLXwh9VosCTgP4mL91kC7hDmqnwTTx6Hu/sys/windows"
	registry "gx/ipfs/QmVGjyM9i2msKvLXwh9VosCTgP4mL91kC7hDmqnwTTx6Hu/sys/windows/registry"
)

const (
	SE_PRIVILEGE_ENABLED_BY_DEFAULT = 0x00000001
	SE_PRIVILEGE_ENABLED            = 0x00000002
	LOAD_LIBRARY_SEARCH_SYSTEM32    = 0x00000800
)

//NOTE: arrays larger than 1 are not needed for our purpose
// as a result, conversion of Go slice to WINAPI variable-sized array, does not need to be implemented here
const ANYSIZE_ARRAY = 1

//WINAPI structures
type Luid struct {
	LowPart  uint32
	HighPart int32
}

type LUID_AND_ATTRIBUTES struct {
	Luid       Luid
	Attributes uint32
}

type TOKEN_PRIVILEGES struct {
	PrivilegeCount uint32
	Privileges     [ANYSIZE_ARRAY]LUID_AND_ATTRIBUTES
}

type PRIVILEGE_SET struct {
	PrivilegeCount uint32
	Control        uint32
	Privilege      [ANYSIZE_ARRAY]LUID_AND_ATTRIBUTES
}

var advapi32 *windows.DLL

func init() {
	platformLinker = winLink
	// Has Developer Mode privileges (Windows 14972+)
	if isOSLinkAware() {
		// Go does not take advantage of this feature prior to 1.11 (c23afa9)
		if !isGoLinkAware() {
			return
		}
		if isDevModeActive() {
			haveLinkCreatePriviledge = true
			return
		}
	}

	// Has UAC SE_CREATE_SYMBOLIC_LINK_NAME privilege (Vista+)
	var err error
	advapi32, err = loadSystemDLL("Advapi32.dll")
	if err != nil {
		return
	}
	defer advapi32.Release()

	if haveUACLinkPrivilege() {
		haveLinkCreatePriviledge = true
	} else {
		haveLinkCreatePriviledge = requestUACLinkPrivilege() //try to gain it
	}
}

func winLink(l tar.Link) error {
	if _, err := os.Lstat(l.Name); err == nil {
		if err = os.Remove(l.Name); err != nil {
			return err
		}
	}

	err := os.Symlink(l.Target, l.Name)
	if err != nil {
		if lErr, ok := err.(*os.LinkError); ok {
			if lErr.Err == syscall.EWINDOWS { // symlinks are not supported by Golang pre-Vista
				return fmt.Errorf("Symlink %q->%q cannot be created, symlinks are not supported on this version of Windows", l.Name, l.Target)
			}
		}
		if haveLinkCreatePriviledge { // other non-privilege errors
			return err
		}
		return fmt.Errorf("Symlink %q->%q cannot be created, user does not have symlink creation privileges (see: https://git.io/vpHKV)", l.Name, l.Target)
	}
	return nil
}

func loadSystemDLL(name string) (*windows.DLL, error) {
	modHandle, err := windows.LoadLibraryEx(name, 0, LOAD_LIBRARY_SEARCH_SYSTEM32)
	if err != nil {
		return nil, err
	}
	return &windows.DLL{Name: name, Handle: modHandle}, nil
}

func havePrivilege(ClientToken windows.Handle, RequiredPrivileges *PRIVILEGE_SET) (ret bool) {
	privilegeCheck(ClientToken, RequiredPrivileges, &ret)
	return ret
}

func haveUACLinkPrivilege() bool {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer token.Close()

	var linkLUID Luid
	if !lookupPrivilegeValue("", "SeCreateSymbolicLinkPrivilege", &linkLUID) {
		return false
	}

	requiredPrivs := &PRIVILEGE_SET{
		PrivilegeCount: 1,
		Control:        0,
		Privilege: [ANYSIZE_ARRAY]LUID_AND_ATTRIBUTES{
			{
				Luid:       linkLUID,
				Attributes: SE_PRIVILEGE_ENABLED_BY_DEFAULT | SE_PRIVILEGE_ENABLED,
			},
		},
	}
	return havePrivilege(windows.Handle(token), requiredPrivs)
}

func requestUACLinkPrivilege() bool {
	procHandle, err := windows.GetCurrentProcess()
	if err != nil {
		return false
	}

	var accessToken windows.Token
	if err := windows.OpenProcessToken(procHandle, windows.TOKEN_QUERY|windows.TOKEN_ADJUST_PRIVILEGES, &accessToken); err != nil {
		return false
	}
	defer accessToken.Close()

	var linkLUID Luid
	if !lookupPrivilegeValue("", "SeCreateSymbolicLinkPrivilege", &linkLUID) {
		return false
	}

	desiredPrivs := &TOKEN_PRIVILEGES{
		PrivilegeCount: 1,
		Privileges: [ANYSIZE_ARRAY]LUID_AND_ATTRIBUTES{
			{
				Luid:       linkLUID,
				Attributes: SE_PRIVILEGE_ENABLED,
			},
		},
	}

	desiredSize := uint32(unsafe.Sizeof(desiredPrivs))

	if !adjustTokenPrivileges(windows.Handle(accessToken), false, desiredPrivs, desiredSize, nil, nil) {
		return false
	}

	return true
}

func isGoLinkAware() bool {
	for _, tag := range build.Default.ReleaseTags {
		if tag == "go1.11" {
			return true
		}
	}
	return false
}

func isOSLinkAware() bool {
	major, _, build := rawWinver()
	if major < 10 {
		return false
	}
	if major == 10 && build < 14972 { // First version to allow symlink creation by regular users, in dev mode
		return false
	}
	return true
}

// TODO: [anyone] Replace with `windows.GetVersion()` when this is resolved: https://github.com/golang/go/issues/17835
func rawWinver() (major, minor, build uint32) {
	type rtlOSVersionInfo struct {
		dwOSVersionInfoSize uint32
		dwMajorVersion      uint32
		dwMinorVersion      uint32
		dwBuildNumber       uint32
		dwPlatformId        uint32
		szCSDVersion        [128]byte
	}

	ntoskrnl := windows.MustLoadDLL("ntoskrnl.exe")
	defer ntoskrnl.Release()
	proc := ntoskrnl.MustFindProc("RtlGetVersion")

	var verStruct rtlOSVersionInfo
	verStruct.dwOSVersionInfoSize = uint32(unsafe.Sizeof(verStruct))
	proc.Call(uintptr(unsafe.Pointer(&verStruct)))

	return verStruct.dwMajorVersion, verStruct.dwMinorVersion, verStruct.dwBuildNumber
}

// see https://docs.microsoft.com/en-us/windows/uwp/get-started/enable-your-device-for-development
func isDevModeActive() bool {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, "SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\AppModelUnlock", registry.READ)
	if err != nil {
		return false
	}

	val, _, err := key.GetIntegerValue("AllowDevelopmentWithoutDevLicense")
	if err != nil {
		return false
	}

	return val != 0
}

//WINAPI wrappers
func privilegeCheck(ClientToken windows.Handle, RequiredPrivileges *PRIVILEGE_SET, pfResult *bool) bool {
	if advapi32 == nil {
		return false
	}

	proc, err := advapi32.FindProc("PrivilegeCheck")
	if err != nil {
		return false
	}

	r1, _, _ := proc.Call(
		uintptr(ClientToken),
		uintptr(unsafe.Pointer(RequiredPrivileges)),
		uintptr(unsafe.Pointer(pfResult)),
	)

	return r1 == 1
}

func lookupPrivilegeValue(lpSystemName, lpName string, lpLuid *Luid) bool {
	if advapi32 == nil {
		return false
	}

	proc, err := advapi32.FindProc("LookupPrivilegeValueW")
	if err != nil {
		return false
	}

	snPtr, err := windows.UTF16PtrFromString(lpSystemName)
	nPtr, err := windows.UTF16PtrFromString(lpName)

	r1, _, _ := proc.Call(uintptr(unsafe.Pointer(snPtr)), uintptr(unsafe.Pointer(nPtr)), uintptr(unsafe.Pointer(lpLuid)))
	return r1 == 1
}

func adjustTokenPrivileges(TokenHandle windows.Handle, DisableAllPrivileges bool, NewState *TOKEN_PRIVILEGES, BufferLength uint32, PreviousState *TOKEN_PRIVILEGES, ReturnLength *uint32) bool {
	if advapi32 == nil {
		return false
	}

	proc, err := advapi32.FindProc("AdjustTokenPrivileges")
	if err != nil {
		return false
	}

	var DisableAll uintptr
	if DisableAllPrivileges {
		DisableAll = 1
	}

	r1, _, err := proc.Call(
		uintptr(TokenHandle),
		DisableAll,
		uintptr(unsafe.Pointer(NewState)),
		uintptr(BufferLength),
		uintptr(unsafe.Pointer(PreviousState)),
		uintptr(unsafe.Pointer(ReturnLength)),
	)

	winErr := err.(syscall.Errno)
	//call success, operation success
	return r1 == 1 && winErr == 0
}
