package dht

import (
	"encoding/json"
	"time"

	u "github.com/jbenet/go-ipfs/util"
)

type logDhtRPC struct {
	Type     string
	Start    time.Time
	End      time.Time
	Duration time.Duration
	RPCCount int
	Success  bool
}

func startNewRPC(name string) *logDhtRPC {
	r := new(logDhtRPC)
	r.Type = name
	r.Start = time.Now()
	return r
}

func (l *logDhtRPC) EndLog() {
	l.End = time.Now()
	l.Duration = l.End.Sub(l.Start)
}

func (l *logDhtRPC) Print() {
	b, err := json.Marshal(l)
	if err != nil {
		u.DOut(err.Error())
	} else {
		u.DOut(string(b))
	}
}
