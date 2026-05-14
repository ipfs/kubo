package harness

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

type event struct {
	timestamp time.Time
	msg       string
}

type events []*event

func (e events) Len() int           { return len(e) }
func (e events) Less(i, j int) bool { return e[i].timestamp.Before(e[j].timestamp) }
func (e events) Swap(i, j int)      { e[i], e[j] = e[j], e[i] }

// TestLogger is a logger for tests.
// It buffers output and only writes the output if the test fails or output is explicitly turned on.
// The purpose of this logger is to allow Go test to run with the verbose flag without printing logs.
// The verbose flag is useful since it streams test progress, but also printing logs makes the output too verbose.
//
// You can also add prefixes that are prepended to each log message, for extra logging context.
//
// This is implemented as a hierarchy of loggers, with children flushing log entries back to parents.
// This works because t.Cleanup() processes entries in LIFO order, so children always flush first.
//
// Obviously this logger should never be used in production systems.
type TestLogger struct {
	parent        *TestLogger
	children      []*TestLogger
	prefixes      []string
	prefixesIface []any
	t             *testing.T
	buf           events
	m             sync.Mutex
	logsEnabled   bool
}

func NewTestLogger(t *testing.T) *TestLogger {
	l := &TestLogger{t: t, buf: make(events, 0)}
	t.Cleanup(l.flush)
	return l
}

func (t *TestLogger) buildPrefix(timestamp time.Time) string {
	d := timestamp.Format("2006-01-02T15:04:05.999999")
	_, file, lineno, _ := runtime.Caller(2)
	file = filepath.Base(file)
	caller := fmt.Sprintf("%s:%d", file, lineno)

	if len(t.prefixes) == 0 {
		return fmt.Sprintf("%s\t%s\t", d, caller)
	}

	prefixes := strings.Join(t.prefixes, ":")
	return fmt.Sprintf("%s\t%s\t%s: ", d, caller, prefixes)
}

func (t *TestLogger) Log(args ...any) {
	timestamp := time.Now()
	e := t.buildPrefix(timestamp) + fmt.Sprint(args...)
	t.add(&event{timestamp: timestamp, msg: e})
}

func (t *TestLogger) Logf(format string, args ...any) {
	timestamp := time.Now()
	e := t.buildPrefix(timestamp) + fmt.Sprintf(format, args...)
	t.add(&event{timestamp: timestamp, msg: e})
}

func (t *TestLogger) Fatal(args ...any) {
	timestamp := time.Now()
	e := t.buildPrefix(timestamp) + fmt.Sprint(append([]any{"fatal: "}, args...)...)
	t.add(&event{timestamp: timestamp, msg: e})
	t.t.FailNow()
}

func (t *TestLogger) Fatalf(format string, args ...any) {
	timestamp := time.Now()
	e := t.buildPrefix(timestamp) + fmt.Sprintf(fmt.Sprintf("fatal: %s", format), args...)
	t.add(&event{timestamp: timestamp, msg: e})
	t.t.FailNow()
}

func (t *TestLogger) add(e *event) {
	t.m.Lock()
	defer t.m.Unlock()
	t.buf = append(t.buf, e)
}

func (t *TestLogger) AddPrefix(prefix string) *TestLogger {
	l := &TestLogger{
		prefixes:      append(t.prefixes, prefix),
		prefixesIface: append(t.prefixesIface, prefix),
		t:             t.t,
		parent:        t,
		logsEnabled:   t.logsEnabled,
	}
	t.m.Lock()
	defer t.m.Unlock()

	t.children = append(t.children, l)
	t.t.Cleanup(l.flush)

	return l
}

func (t *TestLogger) EnableLogs() {
	t.m.Lock()
	defer t.m.Unlock()
	t.logsEnabled = true
	if t.parent != nil {
		if t.parent.logsEnabled {
			t.parent.EnableLogs()
		}
	}
	fmt.Printf("enabling %d children\n", len(t.children))
	for _, c := range t.children {
		if !c.logsEnabled {
			c.EnableLogs()
		}
	}
}

func (t *TestLogger) flush() {
	if t.t.Failed() || t.logsEnabled {
		t.m.Lock()
		defer t.m.Unlock()
		// if this is a child, send the events to the parent
		// the root parent will print all the events in sorted order
		if t.parent != nil {
			for _, e := range t.buf {
				t.parent.add(e)
			}
		} else {
			// we're the root, sort all the events and then print them
			sort.Sort(t.buf)
			fmt.Println()
			fmt.Printf("Logs for test %q:\n\n", t.t.Name())
			for _, e := range t.buf {
				fmt.Println(e.msg)
			}
			fmt.Println()
		}
		t.buf = nil
	}
}
