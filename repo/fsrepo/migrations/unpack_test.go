package migrations

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func TestUnpackArchive(t *testing.T) {
	// Check unrecognized archive type
	err := unpackArchive("", "no-arch-type", "", "", "")
	if err == nil || err.Error() != "unrecognized archive type: no-arch-type" {
		t.Fatal("expected 'unrecognized archive type' error")
	}

	// Test cannot open errors
	err = unpackArchive("no-archive", "tar.gz", "", "", "")
	if err == nil || !strings.HasPrefix(err.Error(), "cannot open archive file") {
		t.Fatal("expected 'cannot open' error, got:", err)
	}
	err = unpackArchive("no-archive", "zip", "", "", "")
	if err == nil || !strings.HasPrefix(err.Error(), "error opening zip reader") {
		t.Fatal("expected 'cannot open' error, got:", err)
	}
}

func TestUnpackTgz(t *testing.T) {
	tmpDir := t.TempDir()

	badTarGzip := filepath.Join(tmpDir, "bad.tar.gz")
	err := os.WriteFile(badTarGzip, []byte("bad-data\n"), 0o644)
	if err != nil {
		panic(err)
	}
	err = unpackTgz(badTarGzip, "", "abc", "abc")
	if err == nil || !strings.HasPrefix(err.Error(), "error opening gzip reader") {
		t.Fatal("expected error opening gzip reader, got:", err)
	}

	testTarGzip := filepath.Join(tmpDir, "test.tar.gz")
	testData := "some data"
	err = writeTarGzipFile(testTarGzip, "testroot", "testfile", testData)
	if err != nil {
		panic(err)
	}

	out := filepath.Join(tmpDir, "out.txt")

	// Test looking for file that is not in archive
	err = unpackTgz(testTarGzip, "testroot", "abc", out)
	if err == nil || err.Error() != "no binary found in archive" {
		t.Fatal("expected 'no binary found in archive' error, got:", err)
	}

	// Test that unpack works.
	err = unpackTgz(testTarGzip, "testroot", "testfile", out)
	if err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != int64(len(testData)) {
		t.Fatal("unpacked file size is", fi.Size(), "expected", len(testData))
	}
}

func TestUnpackZip(t *testing.T) {
	tmpDir := t.TempDir()

	badZip := filepath.Join(tmpDir, "bad.zip")
	err := os.WriteFile(badZip, []byte("bad-data\n"), 0o644)
	if err != nil {
		panic(err)
	}
	err = unpackZip(badZip, "", "abc", "abc")
	if err == nil || !strings.HasPrefix(err.Error(), "error opening zip reader") {
		t.Fatal("expected error opening zip reader, got:", err)
	}

	testZip := filepath.Join(tmpDir, "test.zip")
	testData := "some data"
	err = writeZipFile(testZip, "testroot", "testfile", testData)
	if err != nil {
		panic(err)
	}

	out := filepath.Join(tmpDir, "out.txt")

	// Test looking for file that is not in archive
	err = unpackZip(testZip, "testroot", "abc", out)
	if err == nil || err.Error() != "no binary found in archive" {
		t.Fatal("expected 'no binary found in archive' error, got:", err)
	}

	// Test that unpack works.
	err = unpackZip(testZip, "testroot", "testfile", out)
	if err != nil {
		t.Fatal(err)
	}

	fi, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() != int64(len(testData)) {
		t.Fatal("unpacked file size is", fi.Size(), "expected", len(testData))
	}
}

func writeTarGzipFile(archName, root, fileName, data string) error {
	archFile, err := os.Create(archName)
	if err != nil {
		return err
	}
	defer archFile.Close()
	w := bufio.NewWriter(archFile)

	err = writeTarGzip(root, fileName, data, w)
	if err != nil {
		return err
	}
	// Flush buffered data to file
	if err = w.Flush(); err != nil {
		return err
	}
	// Close tar file
	if err = archFile.Close(); err != nil {
		return err
	}
	return nil
}

func writeTarGzip(root, fileName, data string, w io.Writer) error {
	// gzip writer writes to buffer
	gzw := gzip.NewWriter(w)
	defer gzw.Close()
	// tar writer writes to gzip
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	var err error
	if fileName != "" {
		hdr := &tar.Header{
			Name: path.Join(root, fileName),
			Mode: 0o600,
			Size: int64(len(data)),
		}
		// Write header
		if err = tw.WriteHeader(hdr); err != nil {
			return err
		}
		// Write file body
		if _, err := tw.Write([]byte(data)); err != nil {
			return err
		}
	}

	if err = tw.Close(); err != nil {
		return err
	}
	// Close gzip writer; finish writing gzip data to buffer
	if err = gzw.Close(); err != nil {
		return err
	}
	return nil
}

func writeZipFile(archName, root, fileName, data string) error {
	archFile, err := os.Create(archName)
	if err != nil {
		return err
	}
	defer archFile.Close()
	w := bufio.NewWriter(archFile)

	err = writeZip(root, fileName, data, w)
	if err != nil {
		return err
	}
	// Flush buffered data to file
	if err = w.Flush(); err != nil {
		return err
	}
	// Close zip file
	if err = archFile.Close(); err != nil {
		return err
	}
	return nil
}

func writeZip(root, fileName, data string, w io.Writer) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	// Write file name
	f, err := zw.Create(path.Join(root, fileName))
	if err != nil {
		return err
	}
	// Write file data
	_, err = f.Write([]byte(data))
	if err != nil {
		return err
	}

	// Close zip writer
	if err = zw.Close(); err != nil {
		return err
	}
	return nil
}
