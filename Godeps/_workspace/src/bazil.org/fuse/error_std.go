// +build !darwin

package fuse

func translateGetxattrError(err error) error {
	return err
}
