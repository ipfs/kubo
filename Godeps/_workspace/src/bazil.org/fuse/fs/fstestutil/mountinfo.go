package fstestutil

// MountInfo describes a mounted file system.
type MountInfo struct {
	FSName string
	Type   string
}

// GetMountInfo finds information about the mount at mnt. It is
// intended for use by tests only, and only fetches information
// relevant to the current tests.
func GetMountInfo(mnt string) (*MountInfo, error) {
	return getMountInfo(mnt)
}
