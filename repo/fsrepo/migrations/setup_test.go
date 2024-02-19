package migrations

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ipfs/boxo/blockservice"
	"github.com/ipfs/boxo/exchange/offline"
	"github.com/ipfs/boxo/gateway"
	blocks "github.com/ipfs/go-block-format"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-unixfsnode/data/builder"
	"github.com/ipld/go-car/v2"
	carblockstore "github.com/ipld/go-car/v2/blockstore"
	"github.com/ipld/go-ipld-prime"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
)

var (
	testIpfsDist string
	testServer   *httptest.Server
)

func TestMain(m *testing.M) {
	// Setup test data
	testDataDir := makeTestData()
	defer os.RemoveAll(testDataDir)

	testCar := makeTestCar(testDataDir)
	defer os.RemoveAll(testCar)

	// Setup test gateway
	fd := setupTestGateway(testCar)
	defer fd.Close()

	// Run tests
	os.Exit(m.Run())
}

func makeTestData() string {
	tempDir, err := os.MkdirTemp("", "kubo-migrations-test-*")
	if err != nil {
		panic(err)
	}

	versions := []string{"v1.0.0", "v1.1.0", "v1.1.2", "v2.0.0-rc1", "2.0.0", "v2.0.1"}
	packages := []string{"kubo", "go-ipfs", "fs-repo-migrations", "fs-repo-1-to-2", "fs-repo-2-to-3", "fs-repo-9-to-10", "fs-repo-10-to-11"}

	// Generate fake data
	for _, name := range packages {
		err = os.MkdirAll(filepath.Join(tempDir, name), 0777)
		if err != nil {
			panic(err)
		}

		err = os.WriteFile(filepath.Join(tempDir, name, "versions"), []byte(strings.Join(versions, "\n")+"\n"), 0666)
		if err != nil {
			panic(err)
		}

		for _, version := range versions {
			filename, archName := makeArchivePath(name, name, version, "tar.gz")
			createFakeArchive(filepath.Join(tempDir, filename), archName, false)

			filename, archName = makeArchivePath(name, name, version, "zip")
			createFakeArchive(filepath.Join(tempDir, filename), archName, true)
		}
	}

	return tempDir
}

func createFakeArchive(archName, name string, archZip bool) {
	err := os.MkdirAll(filepath.Dir(archName), 0777)
	if err != nil {
		panic(err)
	}

	fileName := strings.Split(path.Base(name), "_")[0]
	root := fileName

	// Simulate fetching go-ipfs, which has "ipfs" as the name in the archive.
	if fileName == "go-ipfs" || fileName == "kubo" {
		fileName = "ipfs"
	}
	fileName = ExeName(fileName)

	if archZip {
		err = writeZipFile(archName, root, fileName, "FAKE DATA")
	} else {
		err = writeTarGzipFile(archName, root, fileName, "FAKE DATA")
	}
	if err != nil {
		panic(err)
	}
}

// makeTestCar makes a CAR file with the directory [testData]. This code is mostly
// sourced from https://github.com/ipld/go-car/blob/1e2f0bd2c44ee31f48a8f602b25b5671cc0c4687/cmd/car/create.go
func makeTestCar(testData string) string {
	// make a cid with the right length that we eventually will patch with the root.
	hasher, err := multihash.GetHasher(multihash.SHA2_256)
	if err != nil {
		panic(err)
	}
	digest := hasher.Sum([]byte{})
	hash, err := multihash.Encode(digest, multihash.SHA2_256)
	if err != nil {
		panic(err)
	}
	proxyRoot := cid.NewCidV1(uint64(multicodec.DagPb), hash)

	// Make CAR file
	fd, err := os.CreateTemp("", "kubo-migrations-test-*.car")
	if err != nil {
		panic(err)
	}
	defer fd.Close()
	filename := fd.Name()

	rw, err := carblockstore.OpenReadWriteFile(fd, []cid.Cid{proxyRoot}, carblockstore.WriteAsCarV1(true))
	if err != nil {
		panic(err)
	}
	defer rw.Close()

	ctx := context.Background()

	ls := cidlink.DefaultLinkSystem()
	ls.TrustedStorage = true
	ls.StorageReadOpener = func(_ ipld.LinkContext, l ipld.Link) (io.Reader, error) {
		cl, ok := l.(cidlink.Link)
		if !ok {
			return nil, fmt.Errorf("not a cidlink")
		}
		blk, err := rw.Get(ctx, cl.Cid)
		if err != nil {
			return nil, err
		}
		return bytes.NewBuffer(blk.RawData()), nil
	}
	ls.StorageWriteOpener = func(_ ipld.LinkContext) (io.Writer, ipld.BlockWriteCommitter, error) {
		buf := bytes.NewBuffer(nil)
		return buf, func(l ipld.Link) error {
			cl, ok := l.(cidlink.Link)
			if !ok {
				return fmt.Errorf("not a cidlink")
			}
			blk, err := blocks.NewBlockWithCid(buf.Bytes(), cl.Cid)
			if err != nil {
				return err
			}
			return rw.Put(ctx, blk)
		}, nil
	}

	l, _, err := builder.BuildUnixFSRecursive(testData, &ls)
	if err != nil {
		panic(err)
	}

	rcl, ok := l.(cidlink.Link)
	if !ok {
		panic(fmt.Errorf("could not interpret %s", l))
	}

	if err := rw.Finalize(); err != nil {
		panic(err)
	}
	// re-open/finalize with the final root.
	err = car.ReplaceRootsInFile(filename, []cid.Cid{rcl.Cid})
	if err != nil {
		panic(err)
	}

	return filename
}

func setupTestGateway(testCar string) io.Closer {
	blockService, roots, fd, err := newBlockServiceFromCAR(testCar)
	if err != nil {
		panic(err)
	}

	if len(roots) != 1 {
		panic("expected car with 1 root")
	}

	backend, err := gateway.NewBlocksBackend(blockService)
	if err != nil {
		panic(err)
	}
	conf := gateway.Config{
		NoDNSLink:             false,
		DeserializedResponses: false,
	}

	testIpfsDist = "/ipfs/" + roots[0].String()
	testServer = httptest.NewServer(gateway.NewHandler(conf, backend))

	return fd
}

func newBlockServiceFromCAR(filepath string) (blockservice.BlockService, []cid.Cid, io.Closer, error) {
	r, err := os.Open(filepath)
	if err != nil {
		return nil, nil, nil, err
	}

	bs, err := carblockstore.NewReadOnly(r, nil)
	if err != nil {
		_ = r.Close()
		return nil, nil, nil, err
	}

	roots, err := bs.Roots()
	if err != nil {
		return nil, nil, nil, err
	}

	blockService := blockservice.New(bs, offline.Exchange(bs))
	return blockService, roots, r, nil
}
