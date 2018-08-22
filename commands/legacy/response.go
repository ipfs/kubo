package legacy

import (
	"context"
	"io"
	"os"
	"reflect"
	"sync"

	"gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"

	oldcmds "github.com/ipfs/go-ipfs/commands"
)

// responseWrapper wraps Response and implements olcdms.Response.
// It embeds a Response so some methods are taken from that.
type responseWrapper struct {
	cmds.Response

	out interface{}
}

// Request returns a (faked) oldcmds.Request
func (rw *responseWrapper) Request() oldcmds.Request {
	return &requestWrapper{rw.Response.Request(), nil}
}

// Output returns either a <-chan interface{} on which you can receive the
// emitted values, or an emitted io.Reader
func (rw *responseWrapper) Output() interface{} {
	//if not called before
	if rw.out == nil {
		// get first emitted value
		x, err := rw.Next()
		if err != nil {
			return nil
		}
		if e, ok := x.(*cmdkit.Error); ok {
			ch := make(chan interface{})
			log.Error(e)
			close(ch)
			return (<-chan interface{})(ch)
		}

		switch v := x.(type) {
		case io.Reader:
			// if it's a reader, set it
			rw.out = v
		case cmds.Single:
			rw.out = v.Value
		default:
			// if it is something else, create a channel and copy values from next in there
			ch := make(chan interface{})
			rw.out = (<-chan interface{})(ch)

			go func() {
				defer close(ch)
				ch <- v

				for {
					v, err := rw.Next()

					if err == io.EOF || err == context.Canceled {
						return
					}
					if err != nil {
						log.Error(err)
						return
					}

					ch <- v
				}
			}()
		}
	}

	// if we have it already, return existing value
	return rw.out
}

// SetError is an empty stub
func (rw *responseWrapper) SetError(error, cmdkit.ErrorType) {}

// SetOutput is an empty stub
func (rw *responseWrapper) SetOutput(interface{}) {}

// SetLength is an empty stub
func (rw *responseWrapper) SetLength(uint64) {}

// SetCloser is an empty stub
func (rw *responseWrapper) SetCloser(io.Closer) {}

// Close is an empty stub
func (rw *responseWrapper) Close() error { return nil }

// Marshal is an empty stub
func (rw *responseWrapper) Marshal() (io.Reader, error) { return nil, nil }

// Reader is an empty stub
func (rw *responseWrapper) Reader() (io.Reader, error) { return nil, nil }

// Stdout returns os.Stdout
func (rw *responseWrapper) Stdout() io.Writer { return os.Stdout }

// Stderr returns os.Stderr
func (rw *responseWrapper) Stderr() io.Writer { return os.Stderr }

// fakeResponse implements oldcmds.Response and takes a ResponseEmitter
type fakeResponse struct {
	req  oldcmds.Request
	re   cmds.ResponseEmitter
	out  interface{}
	wait chan struct{}
	once sync.Once
}

// Send emits the value(s) stored in r.out on the ResponseEmitter
func (r *fakeResponse) Send(errCh chan<- error) {
	defer close(errCh)

	out := r.Output()
	if out == nil {
		return
	}

	if ch, ok := out.(chan interface{}); ok {
		out = (<-chan interface{})(ch)
	}

	err := r.re.Emit(out)
	errCh <- err
	return
}

// Request returns the oldcmds.Request that belongs to this Response
func (r *fakeResponse) Request() oldcmds.Request {
	return r.req
}

// SetError forwards the call to the underlying ResponseEmitter
func (r *fakeResponse) SetError(err error, code cmdkit.ErrorType) {
	defer r.once.Do(func() { close(r.wait) })
	r.re.SetError(err, code)
}

// Error is an empty stub
func (r *fakeResponse) Error() *cmdkit.Error {
	return nil
}

// SetOutput sets the output variable to the passed value
func (r *fakeResponse) SetOutput(v interface{}) {
	t := reflect.TypeOf(v)
	_, isReader := v.(io.Reader)

	if t != nil && t.Kind() != reflect.Chan && !isReader {
		v = cmds.Single{Value: v}
	}

	r.out = v
	r.once.Do(func() { close(r.wait) })
}

// Output returns the output variable
func (r *fakeResponse) Output() interface{} {
	<-r.wait
	return r.out
}

// SetLength forwards the call to the underlying ResponseEmitter
func (r *fakeResponse) SetLength(l uint64) {
	r.re.SetLength(l)
}

// Length is an empty stub
func (r *fakeResponse) Length() uint64 {
	return 0
}

// Close forwards the call to the underlying ResponseEmitter
func (r *fakeResponse) Close() error {
	return r.re.Close()
}

// SetCloser is an empty stub
func (r *fakeResponse) SetCloser(io.Closer) {}

// Reader is an empty stub
func (r *fakeResponse) Reader() (io.Reader, error) {
	return nil, nil
}

// Marshal is an empty stub
func (r *fakeResponse) Marshal() (io.Reader, error) {
	return nil, nil
}

// Stdout returns os.Stdout
func (r *fakeResponse) Stdout() io.Writer {
	return os.Stdout
}

// Stderr returns os.Stderr
func (r *fakeResponse) Stderr() io.Writer {
	return os.Stderr
}
