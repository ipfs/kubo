package util

import "time"

var TimeFormatIpfs = time.RFC3339Nano

func ParseRFC3339(s string) (time.Time, error) {
	t, err := time.Parse(TimeFormatIpfs, s)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func FormatRFC3339(t time.Time) string {
	return t.UTC().Format(TimeFormatIpfs)
}
