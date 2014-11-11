package log

import "time"

// Time returns a nanosecond-precision timestamp in zone UTC
func Time() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
