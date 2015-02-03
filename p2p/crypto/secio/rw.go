package secio

import (
	"crypto/cipher"
	"errors"
	"fmt"
	"io"
	"sync"

	"crypto/hmac"

	context "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	proto "github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/goprotobuf/proto"
	msgio "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio"
	mpool "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-msgio/mpool"
)

// ErrMACInvalid signals that a MAC verification failed
var ErrMACInvalid = errors.New("MAC verification failed")

// bufPool is a ByteSlicePool for messages. we need buffers because (sadly)
// we cannot encrypt in place-- the user needs their buffer back.
var bufPool = mpool.ByteSlicePool

type etmWriter struct {
	// params
	pool mpool.Pool        // for the buffers with encrypted data
	msg  msgio.WriteCloser // msgio for knowing where boundaries lie
	str  cipher.Stream     // the stream cipher to encrypt with
	mac  HMAC              // the mac to authenticate data with

	sync.Mutex
}

// NewETMWriter Encrypt-Then-MAC
func NewETMWriter(w io.Writer, s cipher.Stream, mac HMAC) msgio.WriteCloser {
	return &etmWriter{msg: msgio.NewWriter(w), str: s, mac: mac, pool: bufPool}
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
	w.Lock()
	defer w.Unlock()

	// encrypt.
	data := w.pool.Get(uint32(len(b))).([]byte)
	data = data[:len(b)] // the pool's buffer may be larger
	w.str.XORKeyStream(data, b)

	// log.Debugf("ENC plaintext (%d): %s %v", len(b), b, b)
	// log.Debugf("ENC ciphertext (%d): %s %v", len(data), data, data)

	// then, mac.
	if _, err := w.mac.Write(data); err != nil {
		return err
	}

	// Sum appends.
	data = w.mac.Sum(data)
	w.mac.Reset()
	// it's sad to append here. our buffers are -- hopefully -- coming from
	// a shared buffer pool, so the append may not actually cause allocation
	// one can only hope. i guess we'll see.

	return w.msg.WriteMsg(data)
}

func (w *etmWriter) Close() error {
	return w.msg.Close()
}

type etmReader struct {
	msgio.Reader
	io.Closer

	// buffer
	buf []byte

	// params
	msg msgio.ReadCloser // msgio for knowing where boundaries lie
	str cipher.Stream    // the stream cipher to encrypt with
	mac HMAC             // the mac to authenticate data with

	sync.Mutex
}

// NewETMReader Encrypt-Then-MAC
func NewETMReader(r io.Reader, s cipher.Stream, mac HMAC) msgio.ReadCloser {
	return &etmReader{msg: msgio.NewReader(r), str: s, mac: mac}
}

func (r *etmReader) NextMsgLen() (int, error) {
	return r.msg.NextMsgLen()
}

func (r *etmReader) drainBuf(buf []byte) int {
	if r.buf == nil {
		return 0
	}

	n := copy(buf, r.buf)
	r.buf = r.buf[n:]
	return n
}

func (r *etmReader) Read(buf []byte) (int, error) {
	r.Lock()
	defer r.Unlock()

	// first, check if we have anything in the buffer
	copied := r.drainBuf(buf)
	buf = buf[copied:]
	if copied > 0 {
		return copied, nil
		// return here to avoid complicating the rest...
		// user can call io.ReadFull.
	}

	// check the buffer has enough space for the next msg
	fullLen, err := r.msg.NextMsgLen()
	if err != nil {
		return 0, err
	}

	buf2 := buf
	changed := false
	// if not enough space, allocate a new buffer.
	if cap(buf) < fullLen {
		buf2 = make([]byte, fullLen)
		changed = true
	}
	buf2 = buf2[:fullLen]

	n, err := io.ReadFull(r.msg, buf2)
	if err != nil {
		return n, err
	}

	m, err := r.macCheckThenDecrypt(buf2)
	if err != nil {
		return 0, err
	}
	buf2 = buf2[:m]
	if !changed {
		return m, nil
	}

	n = copy(buf, buf2)
	if len(buf2) > len(buf) {
		r.buf = buf2[len(buf):] // had some left over? save it.
	}
	return n, nil
}

func (r *etmReader) ReadMsg() ([]byte, error) {
	r.Lock()
	defer r.Unlock()

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
		log.Debug("MAC Invalid:", expected, "!=", macd)
		return 0, ErrMACInvalid
	}

	// ok seems good. decrypt. (can decrypt in place, yay!)
	// log.Debugf("DEC ciphertext (%d): %s %v", len(data), data, data)
	r.str.XORKeyStream(data, data)
	// log.Debugf("DEC plaintext (%d): %s %v", len(data), data, data)

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
