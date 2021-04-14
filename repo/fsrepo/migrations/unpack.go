package migrations

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
)

func unpackArchive(arcPath, atype, root, name, out string) error {
	var err error
	switch atype {
	case "tar.gz":
		err = unpackTgz(arcPath, root, name, out)
	case "zip":
		err = unpackZip(arcPath, root, name, out)
	default:
		err = fmt.Errorf("unrecognized archive type: %s", atype)
	}
	if err != nil {
		return err
	}
	return nil
}

func unpackTgz(arcPath, root, name, out string) error {
	fi, err := os.Open(arcPath)
	if err != nil {
		return fmt.Errorf("cannot open archive file: %w", err)
	}
	defer fi.Close()

	gzr, err := gzip.NewReader(fi)
	if err != nil {
		return fmt.Errorf("error opening gzip reader: %w", err)
	}
	defer gzr.Close()

	var bin io.Reader
	tarr := tar.NewReader(gzr)

	lookFor := root + "/" + name
	for {
		th, err := tarr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("cannot read archive: %w", err)
		}

		if th.Name == lookFor {
			bin = tarr
			break
		}
	}

	if bin == nil {
		return errors.New("no binary found in archive")
	}

	return writeToPath(bin, out)
}

func unpackZip(arcPath, root, name, out string) error {
	zipr, err := zip.OpenReader(arcPath)
	if err != nil {
		return fmt.Errorf("error opening zip reader: %w", err)
	}
	defer zipr.Close()

	lookFor := root + "/" + name
	var bin io.ReadCloser
	for _, fis := range zipr.File {
		if fis.Name == lookFor {
			rc, err := fis.Open()
			if err != nil {
				return fmt.Errorf("error extracting binary from archive: %w", err)
			}

			bin = rc
			break
		}
	}

	if bin == nil {
		return errors.New("no binary found in archive")
	}

	return writeToPath(bin, out)
}

func writeToPath(rc io.Reader, out string) error {
	binfi, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("error creating output file '%s': %w", out, err)
	}
	defer binfi.Close()

	_, err = io.Copy(binfi, rc)

	return err
}
