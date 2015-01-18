package eventlog

import (
	"fmt"
	"io"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go.net/context"

	prelog "github.com/jbenet/go-ipfs/util/prefixlog"
)

// EventLogger extends the StandardLogger interface to allow for log items
// containing structured metadata
type EventLogger interface {
	prelog.StandardLogger

	// Prefix is like PrefixLogger.Prefix. We override it here
	// because the type changes (we return EventLogger).
	// It's what happens when you wrap interfaces.
	Prefix(fmt string, args ...interface{}) EventLogger

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

	EventBegin(ctx context.Context, event string, m ...Loggable) DoneCloser
}

type DoneCloser interface {
	// Done ends the event
	Done()
	// io.Closer is a convenience-alias for Done
	io.Closer
}

// Logger retrieves an event logger by name
func Logger(system string) EventLogger {

	// TODO if we would like to adjust log levels at run-time. Store this event
	// logger in a map (just like the util.Logger impl)
	return &eventLogger{system: system, PrefixLogger: prelog.Logger(system)}
}

// eventLogger implements the EventLogger and wraps a go-logging Logger
type eventLogger struct {
	prelog.PrefixLogger

	system string
	// TODO add log-level
}

func (el *eventLogger) Prefix(fmt string, args ...interface{}) EventLogger {
	l := el.PrefixLogger.Prefix(fmt, args...)
	return &eventLogger{system: el.system, PrefixLogger: l}
}

func (el *eventLogger) EventBegin(ctx context.Context, event string, metadata ...Loggable) DoneCloser {
	start := time.Now()
	el.Event(ctx, fmt.Sprintln(event, "Begin"), metadata...)
	return doneCloserFunc(func() {
		el.Event(ctx, event, append(metadata, LoggableMap(map[string]interface{}{
			"duration": time.Now().Sub(start),
		}))...)
	})
}

func (el *eventLogger) Event(ctx context.Context, event string, metadata ...Loggable) {

	// Collect loggables for later logging
	var loggables []Loggable

	// get any existing metadata from the context
	existing, err := MetadataFromContext(ctx)
	if err != nil {
		existing = Metadata{}
	}
	loggables = append(loggables, existing)

	for _, datum := range metadata {
		loggables = append(loggables, datum)
	}

	e := entry{
		loggables: loggables,
		system:    el.system,
		event:     event,
	}

	e.Log() // TODO replace this when leveled-logs have been implemented
}

type doneCloserFunc func()

func (f doneCloserFunc) Done() {
	f()
}

func (f doneCloserFunc) Close() error {
	f.Done()
	return nil
}
