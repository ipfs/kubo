package secio

import (
	"crypto/cipher"
	"errors"
	"fmt"
	"io"

	"crypto/hmac"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
)

// ErrMACInvalid signals that a MAC verification failed
var ErrMACInvalid = errors.New("MAC verification failed")

type etmWriter struct {
	// params
	msg msgio.WriteCloser
	str cipher.Stream
	mac HMAC
}

// NewETMWriter Encrypt-Then-MAC
func NewETMWriter(w io.Writer, s cipher.Stream, mac HMAC) msgio.WriteCloser {
	return &etmWriter{msg: msgio.NewWriter(w), str: s, mac: mac}
}

// Write writes passed in buffer as a single message.
func (w *etmWriter) Write(b []byte) (int, error) {
	if err := w.WriteMsg(b); err != nil {
		return 0, err
	}
	return len(b), nil
}

// WriteMsg writes the msg in the passed in buffer.
func (w *etmWriter) WriteMsg(b []byte) error {

	// encrypt.
	w.str.XORKeyStream(b, b)

	// then, mac.
	if _, err := w.mac.Write(b); err != nil {
		return err
	}

	// Sum appends.
	b = w.mac.Sum(b)
	w.mac.Reset()
	// it's sad to append here. our buffers are -- hopefully -- coming from
	// a shared buffer pool, so the append may not actually cause allocation
	// one can only hope. i guess we'll see.

	return w.msg.WriteMsg(b)
}

func (w *etmWriter) Close() error {
	return w.msg.Close()
}

type etmReader struct {
	msgio.Reader
	io.Closer

	// params
	msg msgio.ReadCloser
	str cipher.Stream
	mac HMAC
}

// NewETMReader Encrypt-Then-MAC
func NewETMReader(r io.Reader, s cipher.Stream, mac HMAC) msgio.ReadCloser {
	return &etmReader{msg: msgio.NewReader(r), str: s, mac: mac}
}

func (r *etmReader) Read(buf []byte) (int, error) {
	buf2 := buf
	changed := false
	if cap(buf2) < (len(buf) + r.mac.size) {
		buf2 = make([]byte, len(buf)+r.mac.size)
		changed = true
	}

	// WARNING: assumes msg.Read will only read _one_ message. this is what
	// msgio is supposed to do. but msgio may change in the future. may this
	// comment be your guiding light.
	n, err := r.msg.Read(buf2)
	if err != nil {
		return n, err
	}
	buf2 = buf2[:n]

	m, err := r.macCheckThenDecrypt(buf2)
	if err != nil {
		return 0, err
	}
	buf2 = buf2[:m]
	if changed {
		return copy(buf, buf2), nil
	}
	return m, nil
}

func (r *etmReader) ReadMsg() ([]byte, error) {
	msg, err := r.msg.ReadMsg()
	if err != nil {
		return nil, err
	}

	n, err := r.macCheckThenDecrypt(msg)
	if err != nil {
		return nil, err
	}
	return msg[:n], nil
}

func (r *etmReader) macCheckThenDecrypt(m []byte) (int, error) {
	l := len(m)
	if l < r.mac.size {
		return 0, fmt.Errorf("buffer (%d) shorter than MAC size (%d)", l, r.mac.size)
	}

	mark := l - r.mac.size
	data := m[:mark]
	macd := m[mark:]

	r.mac.Write(data)
	expected := r.mac.Sum(nil)
	r.mac.Reset()

	// check mac. if failed, return error.
	if !hmac.Equal(macd, expected) {
		log.Error("MAC Invalid:", expected, "!=", macd)
		return 0, ErrMACInvalid
	}

	// ok seems good. decrypt.
	r.str.XORKeyStream(data, data)
	return mark, nil
}

func (w *etmReader) Close() error {
	return w.msg.Close()
}

// ReleaseMsg signals a buffer can be reused.
func (r *etmReader) ReleaseMsg(b []byte) {
	r.msg.ReleaseMsg(b)
}

// writeMsgCtx is used by the
func writeMsgCtx(ctx context.Context, w msgio.Writer, msg proto.Message) ([]byte, error) {
	enc, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}

	// write in a goroutine so we can exit when our context is cancelled.
	done := make(chan error)
	go func(m []byte) {
		err := w.WriteMsg(m)
		done <- err
	}(enc)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case e := <-done:
		return enc, e
	}
}

func readMsgCtx(ctx context.Context, r msgio.Reader, p proto.Message) ([]byte, error) {
	var msg []byte

	// read in a goroutine so we can exit when our context is cancelled.
	done := make(chan error)
	go func() {
		var err error
		msg, err = r.ReadMsg()
		done <- err
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case e := <-done:
		if e != nil {
			return nil, e
		}
	}

	return msg, proto.Unmarshal(msg, p)
}
