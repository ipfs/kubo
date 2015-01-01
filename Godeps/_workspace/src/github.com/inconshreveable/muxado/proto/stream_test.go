package proto

import (
	"fmt"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/inconshreveable/muxado/proto/frame"
	"io"
	"io/ioutil"
	"testing"
	"time"
)

func TestSendHalfCloseWithZeroWindow(t *testing.T) {
	t.Parallel()

	local, remote := newFakeConnPair()

	s := NewSession(local, NewStream, false, []Extension{})
	s.(*Session).defaultWindowSize = 10

	go func() {
		trans := frame.NewBasicTransport(remote)
		f, err := trans.ReadFrame()
		if err != nil {
			t.Errorf("Failed to read next frame: %v", err)
			return
		}

		_, ok := f.(*frame.RStreamSyn)
		if !ok {
			t.Errorf("Wrong frame type. Got %v, expected %v", f.Type(), frame.TypeStreamSyn)
			return
		}

		f, err = trans.ReadFrame()
		if err != nil {
			t.Errorf("Failed to read next frame: %v", err)
			return
		}

		fr, ok := f.(*frame.RStreamData)
		if !ok {
			t.Errorf("Wrong frame type. Got %v, expected %v", f.Type(), frame.TypeStreamData)
			return
		}

		if fr.Length() != 10 {
			t.Errorf("Wrong data length. Got %v, expected %d", fr.Length(), 10)
			return
		}

		n, err := io.CopyN(ioutil.Discard, fr.Reader(), 10)
		if n != 10 {
			t.Errorf("Wrong read size. Got %d, expected %d", n, 10)
			return
		}

		f, err = trans.ReadFrame()
		if err != nil {
			t.Errorf("Failed to read next frame: %v", err)
			return
		}

		fr, ok = f.(*frame.RStreamData)
		if !ok {
			t.Errorf("Wrong frame type. Got %v, expected %v", f.Type(), frame.TypeStreamData)
			return
		}

		if !fr.Fin() {
			t.Errorf("Wrong frame flags. Expected fin flag to be set.")
			return
		}

		trans.ReadFrame()
	}()

	str, err := s.Open()
	if err != nil {
		t.Fatalf("Failed to open stream: %v", err)
	}

	_, err = str.Write(make([]byte, 10))
	if err != nil {
		t.Fatalf("Failed to write data: %v", err)
	}

	_, err = str.HalfClose([]byte{})
	if err != nil {
		t.Fatalf("Failed to half-close with an empty buffer")
	}
}

func TestDataAfterRst(t *testing.T) {
	local, remote := newFakeConnPair()

	_ = NewSession(local, NewStream, false, []Extension{})
	trans := frame.NewBasicTransport(remote)

	// make sure that we get an RST STREAM_CLOSED
	done := make(chan int)
	go func() {
		defer func() { done <- 1 }()

		f, err := trans.ReadFrame()
		if err != nil {
			t.Errorf("Failed to read frame sent from session: %v", err)
			return
		}

		fr, ok := f.(*frame.RStreamRst)
		if !ok {
			t.Errorf("Frame is not STREAM_RST: %v", f)
			return
		}

		if fr.ErrorCode() != frame.StreamClosed {
			t.Errorf("Error code on STREAM_RST is not STREAM_CLOSED. Got %d, expected %d", fr.ErrorCode(), frame.StreamClosed)
			return
		}
	}()

	fSyn := frame.NewWStreamSyn()
	if err := fSyn.Set(301, 0, 0, false); err != nil {
		t.Fatalf("Failed to make syn frame: %v", err)
	}

	if err := trans.WriteFrame(fSyn); err != nil {
		t.Fatalf("Failed to send syn: %v", err)
	}

	fRst := frame.NewWStreamRst()
	if err := fRst.Set(301, frame.Cancel); err != nil {
		t.Fatal("Failed to make rst frame: %v", err)
	}

	if err := trans.WriteFrame(fRst); err != nil {
		t.Fatalf("Failed to write rst frame: %v", err)
	}

	fData := frame.NewWStreamData()
	if err := fData.Set(301, []byte{0xa, 0xFF}, false); err != nil {
		t.Fatalf("Failed to set data frame")
	}

	trans.WriteFrame(fData)

	<-done
}

func TestFlowControlError(t *testing.T) {
	local, remote := newFakeConnPair()

	s := NewSession(local, NewStream, false, []Extension{})
	s.(*Session).defaultWindowSize = 10
	trans := frame.NewBasicTransport(remote)

	// make sure that we get an RST FLOW_CONTROL_ERROR
	done := make(chan int)
	go func() {
		defer func() { done <- 1 }()

		f, err := trans.ReadFrame()
		if err != nil {
			t.Errorf("Failed to read frame sent from session: %v", err)
			return
		}

		fr, ok := f.(*frame.RStreamRst)
		if !ok {
			t.Errorf("Frame is not STREAM_RST: %v", f)
			return
		}

		if fr.ErrorCode() != frame.FlowControlError {
			t.Errorf("Error code on STREAM_RST is not FLOW_CONTROL_ERROR. Got %d, expected %d", fr.ErrorCode(), frame.FlowControlError)
			return
		}
	}()

	fSyn := frame.NewWStreamSyn()
	if err := fSyn.Set(301, 0, 0, false); err != nil {
		t.Fatalf("Failed to make syn frame: %v", err)
	}

	if err := trans.WriteFrame(fSyn); err != nil {
		t.Fatalf("Failed to send syn: %v", err)
	}

	fData := frame.NewWStreamData()
	if err := fData.Set(301, make([]byte, 11), false); err != nil {
		t.Fatalf("Failed to set data frame")
	}

	trans.WriteFrame(fData)

	<-done
}

func TestTolerateLateFrameAfterRst(t *testing.T) {
	local, remote := newFakeConnPair()

	s := NewSession(local, NewStream, false, []Extension{})
	trans := frame.NewBasicTransport(remote)

	// make sure that we don't get any error on a late frame
	done := make(chan int)
	go func() {
		defer func() { done <- 1 }()
		// read syn
		trans.ReadFrame()
		// read rst
		trans.ReadFrame()

		// should block
		if f, err := trans.ReadFrame(); err != nil {
			t.Errorf("Error reading frame: %v", err)
		} else {
			t.Errorf("Got frame that we shouldn't have read: %v. Type: %v", f, f.Type())
		}
	}()

	str, err := s.Open()
	if err != nil {
		t.Fatalf("failed to open stream")
	}

	str.(*Stream).resetWith(frame.Cancel, fmt.Errorf("cancel"))

	fData := frame.NewWStreamData()
	if err := fData.Set(str.Id(), []byte{0x1, 0x2, 0x3}, false); err != nil {
		t.Fatalf("Failed to set data frame")
	}
	trans.WriteFrame(fData)

	select {
	case <-done:
		t.Fatalf("Stream sent response to late DATA frame")

	case <-time.After(1 * time.Second):
		// ok
	}
}

// Test that we remove a stream from the session if both sides half-close
func TestRemoveAfterHalfClose(t *testing.T) {
	local, remote := newFakeConnPair()
	remote.Discard()

	s := NewSession(local, NewStream, false, []Extension{})
	trans := frame.NewBasicTransport(remote)

	// open stream
	str, err := s.Open()
	if err != nil {
		t.Fatalf("failed to open stream")
	}

	// half close remote side (true means half-close)
	fData := frame.NewWStreamData()
	if err := fData.Set(str.Id(), []byte{0x1, 0x2, 0x3}, true); err != nil {
		t.Fatalf("Failed to set data frame")
	}

	if err := trans.WriteFrame(fData); err != nil {
		t.Fatalf("Failed to write data frame")
	}

	// half-close local side
	str.HalfClose([]byte{0xFF, 0xFE, 0xFD, 0xFC})

	// yield so the stream can process
	time.Sleep(0)

	// verify stream is removed
	if stream, ok := s.(*Session).streams.Get(str.Id()); ok {
		t.Fatalf("Expected stream %d to be removed after both sides half-closed, but found: %v!", str.Id(), stream)

	}
}

// Test that we get a RST if we send a DATA frame after we send a DATA frame with a FIN
func TestDataAfterFin(t *testing.T) {
}
