package common

import (
	"fmt"
	"strings"
)

func MapGetKV(v map[string]interface{}, key string) (interface{}, error) {
	var ok bool
	var mcursor map[string]interface{}
	var cursor interface{} = v

	parts := strings.Split(key, ".")
	for i, part := range parts {
		sofar := strings.Join(parts[:i], ".")

		mcursor, ok = cursor.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%s key is not a map", sofar)
		}

		cursor, ok = mcursor[part]
		if !ok {
			// Construct the current path traversed to print a nice error message
			var path string
			if len(sofar) > 0 {
				path += sofar + "."
			}
			path += part
			return nil, fmt.Errorf("%s not found", path)
		}
	}
	return cursor, nil
}

func MapSetKV(v map[string]interface{}, key string, value interface{}) error {
	var ok bool
	var mcursor map[string]interface{}
	var cursor interface{} = v

	parts := strings.Split(key, ".")
	for i, part := range parts {
		mcursor, ok = cursor.(map[string]interface{})
		if !ok {
			sofar := strings.Join(parts[:i], ".")
			return fmt.Errorf("%s key is not a map", sofar)
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

// Merges the right map into the left map, recursively traversing child maps
// until a non-map value is found.
func MapMergeDeep(left, right map[string]interface{}) map[string]interface{} {
	// We want to alter a copy of the map, not the original
	result := make(map[string]interface{})
	for k, v := range left {
		result[k] = v
	}

	for key, rightVal := range right {
		// If right value is a map
		if rightMap, ok := rightVal.(map[string]interface{}); ok {
			// If key is in left
			if leftVal, found := result[key]; found {
				// If left value is also a map
				if leftMap, ok := leftVal.(map[string]interface{}); ok {
					// Merge nested map
					result[key] = MapMergeDeep(leftMap, rightMap)
					continue
				}
			}
		}

		// Otherwise set new value to result
		result[key] = rightVal
	}

	return result
}
