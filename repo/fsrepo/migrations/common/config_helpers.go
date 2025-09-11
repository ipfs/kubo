package common

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// GetField retrieves a field from a nested config structure using a dot-separated path
// Example: GetField(config, "DNS.Resolvers") returns config["DNS"]["Resolvers"]
func GetField(config map[string]any, path string) (any, bool) {
	parts := strings.Split(path, ".")
	current := config

	for i, part := range parts {
		// Last part - return the value
		if i == len(parts)-1 {
			val, exists := current[part]
			return val, exists
		}

		// Navigate deeper
		next, exists := current[part]
		if !exists {
			return nil, false
		}

		// Ensure it's a map
		nextMap, ok := next.(map[string]any)
		if !ok {
			return nil, false
		}
		current = nextMap
	}

	return nil, false
}

// SetField sets a field in a nested config structure using a dot-separated path
// It creates intermediate maps as needed
func SetField(config map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := config

	for i, part := range parts {
		// Last part - set the value
		if i == len(parts)-1 {
			current[part] = value
			return
		}

		// Navigate or create intermediate maps
		next, exists := current[part]
		if !exists {
			// Create new intermediate map
			newMap := make(map[string]any)
			current[part] = newMap
			current = newMap
		} else {
			// Ensure it's a map
			nextMap, ok := next.(map[string]any)
			if !ok {
				// Can't navigate further, replace with new map
				newMap := make(map[string]any)
				current[part] = newMap
				current = newMap
			} else {
				current = nextMap
			}
		}
	}
}

// DeleteField removes a field from a nested config structure
func DeleteField(config map[string]any, path string) bool {
	parts := strings.Split(path, ".")

	// Handle simple case
	if len(parts) == 1 {
		_, exists := config[parts[0]]
		delete(config, parts[0])
		return exists
	}

	// Navigate to parent
	parentPath := strings.Join(parts[:len(parts)-1], ".")
	parent, exists := GetField(config, parentPath)
	if !exists {
		return false
	}

	parentMap, ok := parent.(map[string]any)
	if !ok {
		return false
	}

	fieldName := parts[len(parts)-1]
	_, exists = parentMap[fieldName]
	delete(parentMap, fieldName)
	return exists
}

// MoveField moves a field from one location to another
func MoveField(config map[string]any, from, to string) error {
	value, exists := GetField(config, from)
	if !exists {
		return fmt.Errorf("source field %s does not exist", from)
	}

	SetField(config, to, value)
	DeleteField(config, from)
	return nil
}

// RenameField renames a field within the same parent
func RenameField(config map[string]any, path, oldName, newName string) error {
	var parent map[string]any
	if path == "" {
		parent = config
	} else {
		p, exists := GetField(config, path)
		if !exists {
			return fmt.Errorf("parent path %s does not exist", path)
		}
		var ok bool
		parent, ok = p.(map[string]any)
		if !ok {
			return fmt.Errorf("parent path %s is not a map", path)
		}
	}

	value, exists := parent[oldName]
	if !exists {
		return fmt.Errorf("field %s does not exist", oldName)
	}

	parent[newName] = value
	delete(parent, oldName)
	return nil
}

// SetDefault sets a field value only if it doesn't already exist
func SetDefault(config map[string]any, path string, value any) {
	if _, exists := GetField(config, path); !exists {
		SetField(config, path, value)
	}
}

// TransformField applies a transformation function to a field value
func TransformField(config map[string]any, path string, transformer func(any) any) error {
	value, exists := GetField(config, path)
	if !exists {
		return fmt.Errorf("field %s does not exist", path)
	}

	newValue := transformer(value)
	SetField(config, path, newValue)
	return nil
}

// EnsureFieldIs checks if a field equals expected value, sets it if missing
func EnsureFieldIs(config map[string]any, path string, expected any) {
	current, exists := GetField(config, path)
	if !exists || current != expected {
		SetField(config, path, expected)
	}
}

// MergeInto merges multiple source fields into a destination map
func MergeInto(config map[string]any, destination string, sources ...string) {
	var destMap map[string]any

	// Get existing destination if it exists
	if existing, exists := GetField(config, destination); exists {
		if m, ok := existing.(map[string]any); ok {
			destMap = m
		}
	}

	// Merge each source
	for _, source := range sources {
		if value, exists := GetField(config, source); exists {
			if sourceMap, ok := value.(map[string]any); ok {
				if destMap == nil {
					destMap = make(map[string]any)
				}
				maps.Copy(destMap, sourceMap)
			}
		}
	}

	if destMap != nil {
		SetField(config, destination, destMap)
	}
}

// CopyField copies a field value to a new location (keeps original)
func CopyField(config map[string]any, from, to string) error {
	value, exists := GetField(config, from)
	if !exists {
		return fmt.Errorf("source field %s does not exist", from)
	}

	SetField(config, to, value)
	return nil
}

// ConvertInterfaceSlice converts []interface{} to []string
func ConvertInterfaceSlice(slice []interface{}) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if str, ok := item.(string); ok {
			result = append(result, str)
		}
	}
	return result
}

// GetOrCreateSection gets or creates a map section in config
func GetOrCreateSection(config map[string]any, path string) map[string]any {
	existing, exists := GetField(config, path)
	if exists {
		if section, ok := existing.(map[string]any); ok {
			return section
		}
	}

	// Create new section
	section := make(map[string]any)
	SetField(config, path, section)
	return section
}

// SafeCastMap safely casts to map[string]any with fallback to empty map
func SafeCastMap(value any) map[string]any {
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return make(map[string]any)
}

// SafeCastSlice safely casts to []interface{} with fallback to empty slice
func SafeCastSlice(value any) []interface{} {
	if s, ok := value.([]interface{}); ok {
		return s
	}
	return []interface{}{}
}

// ReplaceDefaultsWithAuto replaces default values with "auto" in a map
func ReplaceDefaultsWithAuto(values map[string]any, defaults map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range values {
		if vStr, ok := v.(string); ok {
			if replacement, isDefault := defaults[vStr]; isDefault {
				result[k] = replacement
			} else {
				result[k] = vStr
			}
		}
	}
	return result
}

// EnsureSliceContains ensures a slice field contains a value
func EnsureSliceContains(config map[string]any, path string, value string) {
	existing, exists := GetField(config, path)
	if !exists {
		SetField(config, path, []string{value})
		return
	}

	if slice, ok := existing.([]interface{}); ok {
		// Check if value already exists
		for _, item := range slice {
			if str, ok := item.(string); ok && str == value {
				return // Already contains value
			}
		}
		// Add value
		SetField(config, path, append(slice, value))
	} else if strSlice, ok := existing.([]string); ok {
		if !slices.Contains(strSlice, value) {
			SetField(config, path, append(strSlice, value))
		}
	} else {
		// Replace with new slice containing value
		SetField(config, path, []string{value})
	}
}

// ReplaceInSlice replaces old values with new in a slice field
func ReplaceInSlice(config map[string]any, path string, oldValue, newValue string) {
	existing, exists := GetField(config, path)
	if !exists {
		return
	}

	if slice, ok := existing.([]interface{}); ok {
		result := make([]string, 0, len(slice))
		for _, item := range slice {
			if str, ok := item.(string); ok {
				if str == oldValue {
					result = append(result, newValue)
				} else {
					result = append(result, str)
				}
			}
		}
		SetField(config, path, result)
	}
}

// GetMapSection gets a map section with error handling
func GetMapSection(config map[string]any, path string) (map[string]any, error) {
	value, exists := GetField(config, path)
	if !exists {
		return nil, fmt.Errorf("section %s does not exist", path)
	}

	section, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("section %s is not a map", path)
	}

	return section, nil
}

// CloneStringMap clones a map[string]any to map[string]string
func CloneStringMap(m map[string]any) map[string]string {
	result := make(map[string]string, len(m))
	for k, v := range m {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	return result
}

// IsEmptySlice checks if a value is an empty slice
func IsEmptySlice(value any) bool {
	if value == nil {
		return true
	}
	if slice, ok := value.([]interface{}); ok {
		return len(slice) == 0
	}
	if slice, ok := value.([]string); ok {
		return len(slice) == 0
	}
	return false
}
