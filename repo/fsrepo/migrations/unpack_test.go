package migrations

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"io/ioutil"
	"os"
	"path"
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
	tmpDir, err := ioutil.TempDir("", "testunpacktgz")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	badTarGzip := path.Join(tmpDir, "bad.tar.gz")
	err = ioutil.WriteFile(badTarGzip, []byte("bad-data\n"), 0644)
	if err != nil {
		panic(err)
	}
	err = unpackTgz(badTarGzip, "", "abc", "abc")
	if err == nil || !strings.HasPrefix(err.Error(), "error opening gzip reader") {
		t.Fatal("expected error opening gzip reader, got:", err)
	}

	testTarGzip := path.Join(tmpDir, "test.tar.gz")
	testData := "some data"
	err = writeTarGzip(testTarGzip, "testroot", "testfile", testData)
	if err != nil {
		panic(err)
	}

	out := path.Join(tmpDir, "out.txt")

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
	tmpDir, err := ioutil.TempDir("", "testunpackzip")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	badZip := path.Join(tmpDir, "bad.zip")
	err = ioutil.WriteFile(badZip, []byte("bad-data\n"), 0644)
	if err != nil {
		panic(err)
	}
	err = unpackZip(badZip, "", "abc", "abc")
	if err == nil || !strings.HasPrefix(err.Error(), "error opening zip reader") {
		t.Fatal("expected error opening zip reader, got:", err)
	}

	testZip := path.Join(tmpDir, "test.zip")
	testData := "some data"
	err = writeZip(testZip, "testroot", "testfile", testData)
	if err != nil {
		panic(err)
	}

	out := path.Join(tmpDir, "out.txt")

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

func writeTarGzip(archName, root, fileName, data string) error {
	archFile, err := os.Create(archName)
	if err != nil {
		return err
	}
	defer archFile.Close()
	wr := bufio.NewWriter(archFile)

	// gzip writer writes to buffer
	gzw := gzip.NewWriter(wr)
	defer gzw.Close()
	// tar writer writes to gzip
	tw := tar.NewWriter(gzw)
	defer tw.Close()

	if fileName != "" {
		hdr := &tar.Header{
			Name: path.Join(root, fileName),
			Mode: 0600,
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
	// Flush buffered data to file
	if err = wr.Flush(); err != nil {
		return err
	}
	// Close tar file
	if err = archFile.Close(); err != nil {
		return err
	}
	return nil
}

func writeZip(archName, root, fileName, data string) error {
	archFile, err := os.Create(archName)
	if err != nil {
		return err
	}
	defer archFile.Close()
	wr := bufio.NewWriter(archFile)

	zw := zip.NewWriter(wr)
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
	// Flush buffered data to file
	if err = wr.Flush(); err != nil {
		return err
	}
	// Close zip file
	if err = archFile.Close(); err != nil {
		return err
	}
	return nil
}
