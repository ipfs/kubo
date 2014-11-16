package eventlog

import (
	"encoding/json"
	"fmt"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/maybebtc/logrus"
)

type PoliteJSONFormatter struct{}

func (f *PoliteJSONFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	serialized, err := json.Marshal(entry.Data)
	if err != nil {
		return nil, fmt.Errorf("Failed to marshal fields to JSON, %v", err)
	}
	return append(serialized, '\n'), nil
}
