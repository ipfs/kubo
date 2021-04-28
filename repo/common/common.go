package common

import (
	"fmt"
	"regexp"
	"strings"
)

// Find dynamic map key names passed  as Parent["foo"] notation
var bracketsRe = regexp.MustCompile(`\[([^\[\]]*)\]`)

// Normalization for supporting arbitrary dynamic keys with dots:
// Gateway.PublicGateways["gw.example.com"].UseSubdomains
// Pinning.RemoteServices["pins.example.org"].Policies.MFS.Enable
func keyToLookupData(key string) (normalizedKey string, dynamicKeys map[string]string) {
	bracketedKeys := bracketsRe.FindAllString(key, -1)
	dynamicKeys = make(map[string]string, len(bracketedKeys))
	normalizedKey = key
	for i, mapKeySegment := range bracketedKeys {
		mapKey := strings.TrimLeft(mapKeySegment, "[\"")
		mapKey = strings.TrimRight(mapKey, "\"]")
		placeholder := fmt.Sprintf("mapKey%d", i)
		dynamicKeys[placeholder] = mapKey
		normalizedKey = strings.Replace(normalizedKey, mapKeySegment, fmt.Sprintf(".%s", placeholder), 1)
	}
	return normalizedKey, dynamicKeys
}

func MapGetKV(v map[string]interface{}, key string) (interface{}, error) {
	var ok bool
	var mcursor map[string]interface{}
	var cursor interface{} = v

	normalizedKey, dynamicKeys := keyToLookupData(key)
	parts := strings.Split(normalizedKey, ".")
	for i, part := range parts {
		sofar := strings.Join(parts[:i], ".")

		mcursor, ok = cursor.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%q key is not a map", sofar)
		}
		if dynamicPart, ok := dynamicKeys[part]; ok {
			part = dynamicPart
		}
		cursor, ok = mcursor[part]
		if !ok {
			return nil, fmt.Errorf("%q key has no attribute %q", sofar, part)
		}
	}
	return cursor, nil
}

func MapSetKV(v map[string]interface{}, key string, value interface{}) error {
	var ok bool
	var mcursor map[string]interface{}
	var cursor interface{} = v

	normalizedKey, dynamicKeys := keyToLookupData(key)
	parts := strings.Split(normalizedKey, ".")
	for i, part := range parts {
		mcursor, ok = cursor.(map[string]interface{})
		if !ok {
			sofar := strings.Join(parts[:i], ".")
			return fmt.Errorf("%q key is not a map", sofar)
		}
		if dynamicPart, ok := dynamicKeys[part]; ok {
			part = dynamicPart
		}

		// last part? set here
		if i == (len(parts) - 1) {
			mcursor[part] = value
			break
		}

		cursor, ok = mcursor[part]
		if !ok || cursor == nil { // create map if this is empty or is null
			mcursor[part] = map[string]interface{}{}
			cursor = mcursor[part]
		}
	}
	return nil
}
