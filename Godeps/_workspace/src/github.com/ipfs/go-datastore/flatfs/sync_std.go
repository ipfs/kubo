// +build !windows

package flatfs

import "os"

func syncDir(dir string) error {
	dirF, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirF.Close()
	if err := dirF.Sync(); err != nil {
		return err
	}
	return nil
}
