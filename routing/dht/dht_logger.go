package dht

import (
	"encoding/json"
	"time"
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
		log.Debug(err.Error())
	} else {
		log.Debug(string(b))
	}
}

func (l *logDhtRPC) EndAndPrint() {
	l.EndLog()
	l.Print()
}
