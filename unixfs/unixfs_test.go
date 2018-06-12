package unixfs

import (
	"bytes"
	"encoding/hex"
	"testing"

	proto "gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf/proto"

	dag "github.com/ipfs/go-ipfs/merkledag"
	pb "github.com/ipfs/go-ipfs/unixfs/pb"
)

func TestFSNode(t *testing.T) {
	fsn := new(FSNode)
	fsn.Type = TFile
	for i := 0; i < 16; i++ {
		fsn.AddBlockSize(100)
	}
	fsn.RemoveBlockSize(15)

	fsn.Data = make([]byte, 128)

	b, err := fsn.GetBytes()
	if err != nil {
		t.Fatal(err)
	}

	pbn := new(pb.Data)
	err = proto.Unmarshal(b, pbn)
	if err != nil {
		t.Fatal(err)
	}

	ds, err := DataSize(b)
	if err != nil {
		t.Fatal(err)
	}
	nKids := fsn.NumChildren()
	if nKids != 15 {
		t.Fatal("Wrong number of child nodes")
	}

	if ds != (100*15)+128 {
		t.Fatal("Datasize calculations incorrect!")
	}

	nfsn, err := FSNodeFromBytes(b)
	if err != nil {
		t.Fatal(err)
	}

	if nfsn.FileSize() != (100*15)+128 {
		t.Fatal("fsNode FileSize calculations incorrect")
	}
}

func TestPBdataTools(t *testing.T) {
	raw := []byte{0x00, 0x01, 0x02, 0x17, 0xA1}
	rawPB := WrapData(raw)

	pbDataSize, err := DataSize(rawPB)
	if err != nil {
		t.Fatal(err)
	}

	same := len(raw) == int(pbDataSize)
	if !same {
		t.Fatal("WrapData changes the size of data.")
	}

	rawPBBytes, err := UnwrapData(rawPB)
	if err != nil {
		t.Fatal(err)
	}

	same = bytes.Equal(raw, rawPBBytes)
	if !same {
		t.Fatal("Unwrap failed to produce the correct wrapped data.")
	}

	rawPBdata, err := FromBytes(rawPB)
	if err != nil {
		t.Fatal(err)
	}

	isRaw := rawPBdata.GetType() == TRaw
	if !isRaw {
		t.Fatal("WrapData does not create pb.Data_Raw!")
	}

	catFile := []byte("Mr_Meowgie.gif")
	catPBfile := FilePBData(catFile, 17)
	catSize, err := DataSize(catPBfile)
	if catSize != 17 {
		t.Fatal("FilePBData is the wrong size.")
	}
	if err != nil {
		t.Fatal(err)
	}

	dirPB := FolderPBData()
	dir, err := FromBytes(dirPB)
	isDir := dir.GetType() == TDirectory
	if !isDir {
		t.Fatal("FolderPBData does not create a directory!")
	}
	if err != nil {
		t.Fatal(err)
	}
	_, dirErr := DataSize(dirPB)
	if dirErr == nil {
		t.Fatal("DataSize didn't throw an error when taking the size of a directory.")
	}

	catSym, err := SymlinkData("/ipfs/adad123123/meowgie.gif")
	if err != nil {
		t.Fatal(err)
	}

	catSymPB, err := FromBytes(catSym)
	isSym := catSymPB.GetType() == TSymlink
	if !isSym {
		t.Fatal("Failed to make a Symlink.")
	}
	if err != nil {
		t.Fatal(err)
	}

	_, sizeErr := DataSize(catSym)
	if sizeErr == nil {
		t.Fatal("DataSize didn't throw an error when taking the size of a Symlink.")
	}

}

func TestMetadata(t *testing.T) {
	meta := &Metadata{
		MimeType: "audio/aiff",
		Size:     12345,
	}

	_, err := meta.Bytes()
	if err != nil {
		t.Fatal(err)
	}

	metaPB, err := BytesForMetadata(meta)
	if err != nil {
		t.Fatal(err)
	}

	meta, err = MetadataFromBytes(metaPB)
	if err != nil {
		t.Fatal(err)
	}

	mimeAiff := meta.MimeType == "audio/aiff"
	if !mimeAiff {
		t.Fatal("Metadata does not Marshal and Unmarshal properly!")
	}

}

// originally from test/sharness/t0110-gateway-data/foofoo.block, does not have blocksize defined
// contents: {"data":"CAIYBg==","links":[{"Cid":{"/":"QmcJw6x4bQr7oFnVnF6i8SLcJvhXjaxWvj54FYXmZ4Ct6p"},"Name":"","Size":0},{"Cid":{"/":"QmcJw6x4bQr7oFnVnF6i8SLcJvhXjaxWvj54FYXmZ4Ct6p"},"Name":"","Size":0}]}
const noBlocksizeHex = "0A040802180612240A221220CF92FDEFCDC34CAC009C8B05EB662BE0618DB9DE55ECD42785E9EC6712F8DF6512240A221220CF92FDEFCDC34CAC009C8B05EB662BE0618DB9DE55ECD42785E9EC6712F8DF65"

func TestValidatePB(t *testing.T) {
	noBlocksize, err := hex.DecodeString(noBlocksizeHex)
	if err != nil {
		t.Fatal(err)
	}
	nd, err := dag.DecodeProtobuf(noBlocksize)
	if err != nil {
		t.Fatal(err)
	}
	links := nd.Links()
	fd, err := FromBytes(nd.Data())
	if err != nil {
		t.Fatal(err)
	}
	err = ValidatePB(links, fd)
	if err != nil {
		t.Fatalf("valid node (with no blocksizes) failed to validate: %v", err)
	}
	// create no with no blocksize of filesize
	invalid := *fd
	invalid.Filesize = nil
	err = ValidatePB(links, &invalid)
	if err == nil {
		t.Fatalf("invalid node with no blocksize or filesize validated")
	}
	// give node blocksizes
	fd.Blocksizes = []uint64{3, 3}
	// should be ok
	err = ValidatePB(links, fd)
	if err != nil {
		t.Fatalf("valid node failed to validate: %v", err)
	}
	// give node incorrect filesize
	invalid = *fd
	invalid.Filesize = proto.Uint64(8)
	err = ValidatePB(links, &invalid)
	if err == nil {
		t.Fatal("invalid unixfs node (with incorrect filesize) validated")
	}

	// construct a leaf node, copied from WrapData
	leaf := new(pb.Data)
	typ := pb.Data_Raw
	leaf.Data = []byte("abc")
	leaf.Type = &typ
	leaf.Filesize = proto.Uint64(3)

	err = ValidatePB(nil, leaf)
	if err != nil {
		t.Fatalf("valid leaf node failed to validate: %v", err)
	}

	// make filesize incorrect
	invalid = *leaf
	invalid.Filesize = proto.Uint64(8)
	err = ValidatePB(nil, &invalid)
	if err == nil {
		t.Fatal("invalid leaf node (with incorrect filesize) validated")
	}
}
