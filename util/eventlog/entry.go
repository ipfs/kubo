package eventlog

import (
	"time"

	"github.com/jbenet/go-ipfs/util"
	"github.com/maybebtc/logrus"
)

type entry struct {
	loggables []Loggable
	system    string
	event     string
}

// Log logs the event unconditionally (regardless of log level)
// TODO add support for leveled-logs once we decide which levels we want
// for our structured logs
func (e *entry) Log() {
	e.log()
}

// log is a private method invoked by the public Log, Info, Error methods
func (e *entry) log() {
	// accumulate metadata
	accum := Metadata{}
	for _, loggable := range e.loggables {
		accum = DeepMerge(accum, loggable.Loggable())
	}

	// apply final attributes to reserved keys
	// TODO accum["level"] = level
	accum["event"] = e.event
	accum["system"] = e.system
	accum["time"] = util.FormatRFC3339(time.Now())

	// TODO roll our own event logger
	logrus.WithFields(map[string]interface{}(accum)).Info(e.event)
}
