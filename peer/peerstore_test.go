package peer

import (
	"errors"
	"testing"

	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
)

func setupPeer(ps Peerstore, id string, addr string) (Peer, error) {
	tcp, err := ma.NewMultiaddr(addr)
	if err != nil {
		return nil, err
	}

	p := ps.WithIDString(id)
	p.AddAddress(tcp)
	return p, nil
}

func TestPeerstore(t *testing.T) {

	ps := NewPeerstore()

	p11, _ := setupPeer(ps, "11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a31", "/ip4/127.0.0.1/tcp/1234")
	p21, _ := setupPeer(ps, "11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a32", "/ip4/127.0.0.1/tcp/2345")
	// p31, _ := setupPeer("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33", "/ip4/127.0.0.1/tcp/3456")
	// p41, _ := setupPeer("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a34", "/ip4/127.0.0.1/tcp/4567")

	p13, err := ps.Add(p11)
	if err != nil {
		t.Error(err)
	}

	if p13 != p11 {
		t.Error("these should be the same")
	}

	p12, err := ps.FindOrCreate(ID("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a31"))
	if err != nil {
		t.Error(err)
	}

	if p11 != p12 {
		t.Error(errors.New("peers should be the same"))
	}

	p23, err := ps.Add(p21)
	if err != nil {
		t.Error(err)
	}
	if p23 != p21 {
		t.Error("These should be the same")
	}

	p22, err := ps.FindOrCreate(ID("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a32"))
	if err != nil {
		t.Error(err)
	}

	if p21 != p22 {
		t.Error(errors.New("peers should be the same"))
	}

	_, err = ps.FindOrCreate(ID("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a33"))
	if err != nil {
		t.Error(errors.New("should not have an error here"))
	}

	err = ps.Delete(ID("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a31"))
	if err != nil {
		t.Error(err)
	}

	// reconstruct!
	_, err = ps.FindOrCreate(ID("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a31"))
	if err != nil {
		t.Error(errors.New("should not have an error anyway. reconstruct!"))
	}

	p22, err = ps.FindOrCreate(ID("11140beec7b5ea3f0fdbc95d0dd47f3c5bc275da8a32"))
	if err != nil {
		t.Error(err)
	}

	if p21 != p22 {
		t.Error(errors.New("peers should be the same"))
	}

}
