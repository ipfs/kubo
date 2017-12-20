package legacy

import (
	"io"
	"runtime/debug"

	"gx/ipfs/QmUjA7onJjYmZ2TivD83MeCfXf5o1U4ByVJWPjRru5NA4t/go-ipfs-cmds"

	oldcmds "github.com/ipfs/go-ipfs/commands"
)

// MarshalerEncoder implements Encoder from a Marshaler
type MarshalerEncoder struct {
	m   oldcmds.Marshaler
	w   io.Writer
	req *cmds.Request
}

// NewMarshalerEncoder returns a new MarshalerEncoder
func NewMarshalerEncoder(req *cmds.Request, m oldcmds.Marshaler, w io.Writer) *MarshalerEncoder {
	me := &MarshalerEncoder{
		m:   m,
		w:   w,
		req: req,
	}

	return me
}

// Encode encodes v onto the io.Writer w using Marshaler m, with both m and w passed in NewMarshalerEncoder
func (me *MarshalerEncoder) Encode(v interface{}) error {
	re, res := cmds.NewChanResponsePair(me.req)
	go re.Emit(v)

	r, err := me.m(&responseWrapper{Response: res})
	if err != nil {
		return err
	}
	if r == nil {
		// behave like empty reader
		return nil
	}

	_, err = io.Copy(me.w, r)
	return err
}

func OldContext(env interface{}) *oldcmds.Context {
	ctx, ok := env.(*oldcmds.Context)
	if !ok {
		log.Errorf("OldContext: env passed is not %T but %T\n%s", ctx, env, debug.Stack())
	}

	return ctx
}

/*
// OldReqLog returns an oldcmds.ReqLog from a ReqLog
func OldReqLog(newrl *ReqLog) *oldcmds.ReqLog {
	if newrl == nil {
		return nil
	}

	rl := &oldcmds.ReqLog{}

	for _, rle := range newrl.Requests {
		oldrle := &oldcmds.ReqLogEntry{
			StartTime: rle.StartTime,
			EndTime:   rle.EndTime,
			Active:    rle.Active,
			Command:   rle.Command,
			Options:   rle.Options,
			Args:      rle.Args,
			ID:        rle.ID,
		}
		rl.AddEntry(oldrle)
	}

	return rl
}
*/
