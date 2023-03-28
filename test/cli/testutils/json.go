package testutils

import "encoding/json"

type JSONObj map[string]interface{}

func ToJSONStr(m JSONObj) string {
	b, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return string(b)
}
