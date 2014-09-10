// +build !darwin

package fuse

func translateGetxattrError(err Error) Error {
	return err
}
