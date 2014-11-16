package logrus

import (
	"encoding/json"
	"fmt"
)

type PoliteJSONFormatter struct{}

func (f *PoliteJSONFormatter) Format(entry *Entry) ([]byte, error) {
	serialized, err := json.Marshal(entry.Data)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal fields to JSON, %v", err)
	}
	return append(serialized, '\n'), nil
}
