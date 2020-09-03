package integrationtest

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/ipfs/go-bitswap/decision"
	config "github.com/ipfs/go-ipfs-config"
	"github.com/ipfs/go-ipfs/core"
	mock "github.com/ipfs/go-ipfs/core/mock"
	"github.com/ipfs/go-ipfs/plugin"
	peer "github.com/libp2p/go-libp2p-core/peer"
)

func TestScoreLedgerNotLoaded(t *testing.T) {
	n, err := nodeWithNamedScoreLedger("testledger")
	if err == nil {
		n.Close()
		t.Fatal("Expected to fail")
	}
	if err != nil {
		if err.Error() != "score ledger 'testledger' not found, check if plugin is loaded" {
			t.Fatal("Unexpected error message")
		}
	}
}

func TestScoreLedgerLoadStartStop(t *testing.T) {
	tp := &testingScoreLedgerPlugin{
		ledger: newTestingScoreLedger(),
	}
	plugins, err := loadPlugins(tp)
	if err != nil {
		t.Fatal(err)
	}
	defer plugins.Close()

	n, err := nodeWithNamedScoreLedger("testledger")
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()

	select {
	case <-tp.ledger.started:
		if tp.ledger.scorePeer == nil {
			t.Fatal("Expected the score function to be initialized")
		}
	case <-time.After(time.Second * 5):
		t.Fatal("Expected the score ledger to be started within 5s")
	}

	n.Close()
	select {
	case <-tp.ledger.closed:
	case <-time.After(time.Second * 5):
		t.Fatal("Expected the score ledger to be closed within 5s")
	}
}

func nodeWithNamedScoreLedger(name string) (*core.IpfsNode, error) {
	cfg, err := config.Init(ioutil.Discard, 2048)
	if err != nil {
		return nil, err
	}
	cfg.Experimental.WithScoreLedger = name
	return mock.NewMockNodeWithConfig(cfg)
}

type testingScoreLedger struct {
	scorePeer decision.ScorePeerFunc
	started   chan struct{}
	closed    chan struct{}
}

func newTestingScoreLedger() *testingScoreLedger {
	return &testingScoreLedger{
		nil,
		make(chan struct{}),
		make(chan struct{}),
	}
}

func (tsl *testingScoreLedger) GetReceipt(p peer.ID) *decision.Receipt {
	return nil
}
func (tsl *testingScoreLedger) AddToSentBytes(p peer.ID, n int)     {}
func (tsl *testingScoreLedger) AddToReceivedBytes(p peer.ID, n int) {}
func (tsl *testingScoreLedger) PeerConnected(p peer.ID)             {}
func (tsl *testingScoreLedger) PeerDisconnected(p peer.ID)          {}
func (tsl *testingScoreLedger) Start(scorePeer decision.ScorePeerFunc) {
	tsl.scorePeer = scorePeer
	close(tsl.started)
}
func (tsl *testingScoreLedger) Stop() {
	close(tsl.closed)
}

type testingScoreLedgerPlugin struct {
	ledger *testingScoreLedger
}

func (tp *testingScoreLedgerPlugin) Name() string    { return "testledger" }
func (tp *testingScoreLedgerPlugin) Version() string { return "0.0.0" }
func (tp *testingScoreLedgerPlugin) Init(env *plugin.Environment) error {
	return nil
}
func (tp *testingScoreLedgerPlugin) Ledger() (decision.ScoreLedger, error) {
	return tp.ledger, nil
}
