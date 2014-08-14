package dht

import (
	"encoding/json"
	"time"

	u "github.com/jbenet/go-ipfs/util"
)

type logDhtRpc struct {
	Type     string
	Start    time.Time
	End      time.Time
	Duration time.Duration
	RpcCount int
	Success  bool
}

func startNewRpc(name string) *logDhtRpc {
	r := new(logDhtRpc)
	r.Type = name
	r.Start = time.Now()
	return r
}

func (l *logDhtRpc) EndLog() {
	l.End = time.Now()
	l.Duration = l.End.Sub(l.Start)
}

func (l *logDhtRpc) Print() {
	b, err := json.Marshal(l)
	if err != nil {
		u.POut(err.Error())
	} else {
		u.POut(string(b))
	}
}
