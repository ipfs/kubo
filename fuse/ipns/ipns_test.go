package ipns

import (
	"crypto/rand"
	"os"
	"testing"

	fstest "github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs/fstestutil"
)

func randBytes(size int) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}

func TestIpnsBasicIO(t *testing.T) {
	fs, err := NewIpns(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	mnt, err := fstest.MountedT(t, fs)
	if err != nil {
		t.Fatal(err)
	}

	data := randBytes(12345)
	fi, err := os.Create(mnt.Dir + "/local/testfile")
	if err != nil {
		t.Fatal(err)
	}

	n, err := fi.Write(data)
	if err != nil {
		t.Fatal(err)
	}

	if n != len(data) {
		t.Fatal("Didnt write proper amount!")
	}

	fi.Close()

	//TODO: maybe wait for the publish to happen? or not, should test both cases

	fi, err = os.Open(mnt.Dir + "/local/testfile")
	if err != nil {
		t.Fatal(err)
	}

	rbuf := make([]byte, len(data))
	n, err = fi.Read(rbuf)
	if err != nil {
		t.Fatal(err)
	}

	if n != len(rbuf) {
		t.Fatal("Failed to read correct amount!")
	}

	fi.Close()
}
