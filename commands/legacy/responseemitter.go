package legacy

import (
	"fmt"
	"io"

	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"

	oldcmds "github.com/ipfs/go-ipfs/commands"
)

// wrappedResponseEmitter implements a ResponseEmitter by forwarding everything to an oldcmds.Response
type wrappedResponseEmitter struct {
	r oldcmds.Response
}

// SetLength forwards the call to the underlying oldcmds.Response
func (re *wrappedResponseEmitter) SetLength(l uint64) {
	re.r.SetLength(l)
}

// SetError forwards the call to the underlying oldcmds.Response
func (re *wrappedResponseEmitter) SetError(err interface{}, code cmdkit.ErrorType) {
	re.r.SetError(fmt.Errorf("%v", err), code)
}

// Close forwards the call to the underlying oldcmds.Response
func (re *wrappedResponseEmitter) Close() error {
	return re.r.Close()
}

// Emit sends the value to the underlying oldcmds.Response
func (re *wrappedResponseEmitter) Emit(v interface{}) error {
	if re.r.Output() == nil {
		switch c := v.(type) {
		case io.Reader:
			re.r.SetOutput(c)
			return nil
		case chan interface{}:
			re.r.SetOutput(c)
			return nil
		case <-chan interface{}:
			re.r.SetOutput(c)
			return nil
		default:
			re.r.SetOutput(make(chan interface{}))
		}
	}

	go func() {
		re.r.Output().(chan interface{}) <- v
	}()

	return nil
}
