package proto

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/inconshreveable/muxado/proto/frame"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"testing"
	"time"
)

func fakeStreamFactory(id frame.StreamId, priority frame.StreamPriority, streamType frame.StreamType, finLocal bool, finRemote bool, windowSize uint32, sess session) stream {
	return new(fakeStream)
}

type fakeStream struct {
}

func (s *fakeStream) Write([]byte) (int, error)               { return 0, nil }
func (s *fakeStream) Read([]byte) (int, error)                { return 0, nil }
func (s *fakeStream) Close() error                            { return nil }
func (s *fakeStream) SetDeadline(time.Time) error             { return nil }
func (s *fakeStream) SetReadDeadline(time.Time) error         { return nil }
func (s *fakeStream) SetWriteDeadline(time.Time) error        { return nil }
func (s *fakeStream) HalfClose([]byte) (int, error)           { return 0, nil }
func (s *fakeStream) Id() frame.StreamId                      { return 0 }
func (s *fakeStream) StreamType() frame.StreamType            { return 0 }
func (s *fakeStream) Session() ISession                       { return nil }
func (s *fakeStream) RemoteAddr() net.Addr                    { return nil }
func (s *fakeStream) LocalAddr() net.Addr                     { return nil }
func (s *fakeStream) handleStreamData(*frame.RStreamData)     {}
func (s *fakeStream) handleStreamWndInc(*frame.RStreamWndInc) {}
func (s *fakeStream) handleStreamRst(*frame.RStreamRst)       {}
func (s *fakeStream) closeWith(error)                         {}

type fakeConn struct {
	in     *io.PipeReader
	out    *io.PipeWriter
	closed bool
}

func (c *fakeConn) SetDeadline(time.Time) error      { return nil }
func (c *fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *fakeConn) SetWriteDeadline(time.Time) error { return nil }
func (c *fakeConn) LocalAddr() net.Addr              { return nil }
func (c *fakeConn) RemoteAddr() net.Addr             { return nil }
func (c *fakeConn) Close() error                     { c.closed = true; c.in.Close(); return c.out.Close() }
func (c *fakeConn) Read(p []byte) (int, error)       { return c.in.Read(p) }
func (c *fakeConn) Write(p []byte) (int, error)      { return c.out.Write(p) }
func (c *fakeConn) Discard()                         { go io.Copy(ioutil.Discard, c.in) }

func newFakeConnPair() (local *fakeConn, remote *fakeConn) {
	local, remote = new(fakeConn), new(fakeConn)
	local.in, remote.out = io.Pipe()
	remote.in, local.out = io.Pipe()
	return
}

func TestFailWrongClientParity(t *testing.T) {
	t.Parallel()

	local, remote := newFakeConnPair()

	// don't need the remote output
	remote.Discard()

	// false for a server session
	s := NewSession(local, fakeStreamFactory, false, []Extension{})

	// 300 is even, and only servers send even stream ids
	f := frame.NewWStreamSyn()
	f.Set(300, 0, 0, false)

	// send the frame into the session
	trans := frame.NewBasicTransport(remote)
	trans.WriteFrame(f)

	// wait for failure
	code, err, _ := s.Wait()

	if code != frame.ProtocolError {
		t.Errorf("Session not terminated with protocol error. Got %d, expected %d. Session error: %v", code, frame.ProtocolError, err)
	}

	if !local.closed {
		t.Errorf("Session transport not closed after protocol failure.")
	}
}

func TestWrongServerParity(t *testing.T) {
	t.Parallel()

	local, remote := newFakeConnPair()

	// true for a client session
	s := NewSession(local, fakeStreamFactory, true, []Extension{})

	// don't need the remote output
	remote.Discard()

	// 300 is even, and only servers send even stream ids
	f := frame.NewWStreamSyn()
	f.Set(301, 0, 0, false)

	// send the frame into the session
	trans := frame.NewBasicTransport(remote)
	trans.WriteFrame(f)

	// wait for failure
	code, err, _ := s.Wait()

	if code != frame.ProtocolError {
		t.Errorf("Session not terminated with protocol error. Got %d, expected %d. Session error: %v", code, frame.ProtocolError, err)
	}

	if !local.closed {
		t.Errorf("Session transport not closed after protocol failure.")
	}
}

func TestAcceptStream(t *testing.T) {
	t.Parallel()

	local, remote := newFakeConnPair()

	// don't need the remote output
	remote.Discard()

	// true for a client session
	s := NewSession(local, NewStream, true, []Extension{})
	defer s.Close()

	f := frame.NewWStreamSyn()
	f.Set(300, 0, 0, false)

	// send the frame into the session
	trans := frame.NewBasicTransport(remote)
	trans.WriteFrame(f)

	done := make(chan int)
	go func() {
		defer func() { done <- 1 }()

		// wait for accept
		str, err := s.Accept()

		if err != nil {
			t.Errorf("Error accepting stream: %v", err)
			return
		}

		if str.Id() != frame.StreamId(300) {
			t.Errorf("Stream has wrong id. Expected %d, got %d", str.Id(), 300)
		}
	}()

	select {
	case <-time.After(time.Second):
		t.Fatalf("Timed out!")
	case <-done:
	}
}

func TestSynLowId(t *testing.T) {
	t.Parallel()

	local, remote := newFakeConnPair()

	// don't need the remote output
	remote.Discard()

	// true for a client session
	s := NewSession(local, fakeStreamFactory, true, []Extension{})

	// Start a stream
	f := frame.NewWStreamSyn()
	f.Set(302, 0, 0, false)

	// send the frame into the session
	trans := frame.NewBasicTransport(remote)
	trans.WriteFrame(f)

	// accept it
	s.Accept()

	// Start a closed stream at a lower id number
	f.Set(300, 0, 0, false)

	// send the frame into the session
	trans.WriteFrame(f)

	code, err, _ := s.Wait()
	if code != frame.ProtocolError {
		t.Errorf("Session not terminated with protocol error, got %d expected %d. Error: %v", code, frame.ProtocolError, err)
	}
}

// Check that sending a frame of the wrong size responds with FRAME_SIZE_ERROR
func TestFrameSizeError(t *testing.T) {
}

// Check that we get a protocol error for sending STREAM_DATA on a stream id that was never opened
func TestDataOnClosed(t *testing.T) {
}

// Check that we get nothing for sending STREAM_WND_INC on a stream id that was never opened
func TestWndIncOnClosed(t *testing.T) {
}

// Check that we get nothing for sending STREAM_RST on a stream id that was never opened
func TestRstOnClosed(t *testing.T) {
}

func TestGoAway(t *testing.T) {
}

func TestCloseGoAway(t *testing.T) {
}

func TestKill(t *testing.T) {
}

// make sure we get a valid syn frame from opening a new stream
func TestOpen(t *testing.T) {
}

// test opening a new stream that is immediately half-closed
func TestOpenWithFin(t *testing.T) {
}

// validate that a session fulfills the net.Listener interface
// compile-only check
func TestNetListener(t *testing.T) {
	t.Parallel()

	_ = func() {
		s := NewSession(new(fakeConn), NewStream, false, []Extension{})
		http.Serve(s.NetListener(), nil)
	}
}

func TestNetListenerAccept(t *testing.T) {
	t.Parallel()
	local, remote := newFakeConnPair()

	sLocal := NewSession(local, NewStream, false, []Extension{})
	sRemote := NewSession(remote, NewStream, true, []Extension{})

	go func() {
		_, err := sRemote.Open()
		if err != nil {
			t.Errorf("Failed to open stream: %v", err)
			return
		}
	}()

	l := sLocal.NetListener()

	_, err := l.Accept()
	if err != nil {
		t.Fatalf("Failed to accept stream: %v", err)
	}
}

// set up a fake extension which tries to accept a stream.
// we're testing to make sure that when the remote side closes the connection
// that the extension actually gets an error back from its accept() method
type fakeExt struct {
	closeOk chan int
}

func (e *fakeExt) Start(sess ISession, accept ExtAccept) frame.StreamType {
	go func() {
		_, err := accept()
		if err != nil {
			// we should get an error when the session close
			e.closeOk <- 1
		}
	}()

	return MinExtensionType
}

func TestExtensionCleanupAccept(t *testing.T) {
	t.Parallel()
	local, remote := newFakeConnPair()

	closeOk := make(chan int)
	_ = NewSession(local, NewStream, false, []Extension{&fakeExt{closeOk}})
	sRemote := NewSession(remote, NewStream, true, []Extension{})

	sRemote.Close()

	select {
	case <-time.After(time.Second):
		t.Fatalf("Timed out!")
	case <-closeOk:
	}
}

func TestWriteAfterClose(t *testing.T) {
	t.Parallel()
	local, remote := newFakeConnPair()
	sLocal := NewSession(local, NewStream, false, []Extension{})
	sRemote := NewSession(remote, NewStream, true, []Extension{})

	closed := make(chan int)
	go func() {
		stream, err := sRemote.Open()
		if err != nil {
			t.Errorf("Failed to open stream: %v", err)
			return
		}

		<-closed
		if _, err = stream.Write([]byte("test!")); err != nil {
			t.Errorf("Failed to write test data: %v", err)
			return
		}

		if _, err := sRemote.Open(); err != nil {
			t.Errorf("Failed to open second stream: %v", err)
			return
		}
	}()

	stream, err := sLocal.Accept()
	if err != nil {
		t.Fatalf("Failed to accept stream!")
	}

	// tell the other side that we closed so they can write late
	stream.Close()
	closed <- 1

	if _, err = sLocal.Accept(); err != nil {
		t.Fatalf("Failed to accept second connection: %v", err)
	}
}
