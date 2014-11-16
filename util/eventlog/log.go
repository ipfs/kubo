package eventlog

import (
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/Sirupsen/logrus"
	logging "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-logging"
	"github.com/jbenet/go-ipfs/util"
)

// EventLogger extends the StandardLogger interface to allow for log items
// containing structured metadata
type EventLogger interface {
	StandardLogger

	// Event merges structured data from the provided inputs into a single
	// machine-readable log event.
	//
	// If the context contains metadata, a copy of this is used as the base
	// metadata accumulator.
	//
	// If one or more loggable objects are provided, these are deep-merged into base blob.
	//
	// Next, the event name is added to the blob under the key "event". If
	// the key "event" already exists, it will be over-written.
	//
	// Finally the timestamp and package name are added to the accumulator and
	// the metadata is logged.
	Event(ctx context.Context, event string, m ...Loggable)
}

// StandardLogger provides API compatibility with standard printf loggers
// eg. go-logging
type StandardLogger interface {
	Critical(args ...interface{})
	Criticalf(format string, args ...interface{})
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Notice(args ...interface{})
	Noticef(format string, args ...interface{})
	Panic(args ...interface{})
	Panicf(format string, args ...interface{})
	Warning(args ...interface{})
	Warningf(format string, args ...interface{})
}

// Logger retrieves an event logger by name
func Logger(system string) EventLogger {

	// TODO if we would like to adjust log levels at run-time. Store this event
	// logger in a map (just like the util.Logger impl)

	return &eventLogger{system: system, Logger: util.Logger(system)}
}

// eventLogger implements the EventLogger and wraps a go-logging Logger
type eventLogger struct {
	*logging.Logger
	system string
}

func (el *eventLogger) Event(ctx context.Context, event string, metadata ...Loggable) {

	// get any existing metadata from the context
	existing, err := MetadataFromContext(ctx)
	if err != nil {
		existing = Metadata{}
	}

	// accumulate metadata
	accum := existing
	for _, datum := range metadata {
		accum = DeepMerge(accum, datum.Loggable())
	}

	// apply final attributes to reserved keys
	accum["event"] = event
	accum["system"] = el.system
	accum["time"] = util.FormatRFC3339(time.Now())

	// TODO roll our own event logger
	logrus.WithFields(map[string]interface{}(accum)).Info(event)
}
