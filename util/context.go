package util

import (
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

// privateChanType protects the channel. Since this is a package-private type,
// only methods defined in this package can get the error value from the
// context.
type privateChanType chan error

const errLogKey = "the key used to extract the error log from the context"

// ContextWithErrorLog returns a copy of parent and an error channel that can
// be used to receive errors sent with the LogError method.
func ContextWithErrorLog(parent context.Context) (context.Context, <-chan error) {
	errs := make(privateChanType)
	ctx := context.WithValue(parent, errLogKey, errs)
	return ctx, errs
}

// LogError logs the error to the owner of the context.
//
// If this context was created with ContextWithErrorLog, then this method
// passes the error to context creator over an unbuffered channel.
//
// If this context was created by other means, this method is a no-op.
func LogError(ctx context.Context, err error) {
	v := ctx.Value(errLogKey)
	errs, ok := v.(privateChanType)
	if !ok {
		return
	}
	errs <- err
}
