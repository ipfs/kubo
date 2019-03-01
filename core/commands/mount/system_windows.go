package fusemount

import (
	"unsafe"

	"gx/ipfs/QmVGjyM9i2msKvLXwh9VosCTgP4mL91kC7hDmqnwTTx6Hu/sys/windows"

	"github.com/billziss-gh/cgofuse/fuse"
)

//try to extract these into other pkg(s)
const LOAD_LIBRARY_SEARCH_SYSTEM32 = 0x00000800

func loadSystemDLL(name string) (*windows.DLL, error) {
	modHandle, err := windows.LoadLibraryEx(name, 0, LOAD_LIBRARY_SEARCH_SYSTEM32)
	if err != nil {
		return nil, err
	}
	return &windows.DLL{Name: name, Handle: modHandle}, nil
}

func (fs *FUSEIPFS) fuseFreeSize(fStatfs *fuse.Statfs_t, path string) error {
	mod, err := loadSystemDLL("kernel32.dll")
	if err != nil {
		return err
	}
	defer mod.Release()
	proc, err := mod.FindProc("GetDiskFreeSpaceExW")
	if err != nil {
		return err
	}

	var (
		FreeBytesAvailableToCaller,
		TotalNumberOfBytes,
		TotalNumberOfFreeBytes uint64

		SectorsPerCluster,
		BytesPerSector uint16
		//NumberOfFreeClusters,
		//TotalNumberOfClusters uint16
	)
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err //check syscall.EINVAL in caller; NUL byte in string
	}

	r1, _, wErr := proc.Call(uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&FreeBytesAvailableToCaller)),
		uintptr(unsafe.Pointer(&TotalNumberOfBytes)),
		uintptr(unsafe.Pointer(&TotalNumberOfFreeBytes)),
	)
	if r1 == 0 {
		return wErr
	}

	proc, _ = mod.FindProc("GetDiskFreeSpaceW")
	r1, _, wErr = proc.Call(uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&SectorsPerCluster)),
		uintptr(unsafe.Pointer(&BytesPerSector)),
		//uintptr(unsafe.Pointer(&NumberOfFreeClusters)),
		0,
		//uintptr(unsafe.Pointer(&TotalNumberOfClusters)),
		0,
	)
	if r1 == 0 {
		return wErr
	}

	fStatfs.Bsize = uint64(SectorsPerCluster * BytesPerSector)
	fStatfs.Frsize = uint64(BytesPerSector)
	fStatfs.Blocks = TotalNumberOfBytes / uint64(BytesPerSector)
	fStatfs.Bfree = TotalNumberOfFreeBytes / (uint64(BytesPerSector))
	fStatfs.Bavail = FreeBytesAvailableToCaller / (uint64(BytesPerSector))
	fStatfs.Files = ^uint64(0)
	fStatfs.Ffree = fs.AvailableHandles(aFiles)
	fStatfs.Favail = fStatfs.Ffree

	/* TODO
	fStatfs.Fsid
	fStatfs.Flag
	fStatfs.Namemax
	*/

	return nil
}
