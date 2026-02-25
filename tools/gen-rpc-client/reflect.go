package main

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// FieldInfo describes one field of a generated response struct.
type FieldInfo struct {
	Name    string // Go field name
	Type    string // Go type expression
	JSONTag string // json struct tag value
}

// TypeInfo describes a generated response struct.
type TypeInfo struct {
	Name   string      // e.g. "PinAddResponse"
	Fields []FieldInfo // exported fields
}

// knownTypeMap maps specific Go types to simpler types for the generated
// client. Keys use the full package path + "." + type name.
var knownTypeMap = map[string]string{}

// knownTypePatterns maps type name suffixes (last pkg segment + name) to
// simpler types. This handles cases where the full package path varies.
var knownTypePatterns = map[string]string{
	"Cid":           "string",
	"ID":            "string",
	"Multiaddr":     "string",
	"Path":          "string",
	"ImmutablePath": "string",
	"Duration":      "string",
	"FileMode":      "string",
}

// knownFullPaths maps full "pkgpath.TypeName" to simpler types.
var knownFullPaths = map[string]string{
	"github.com/ipfs/go-cid.Cid":                     "string",
	"github.com/libp2p/go-libp2p/core/peer.ID":       "string",
	"github.com/multiformats/go-multiaddr.Multiaddr": "string",
	"github.com/libp2p/go-libp2p/core/protocol.ID":   "string",
	"github.com/ipfs/boxo/path.Path":                 "string",
	"github.com/ipfs/boxo/path.ImmutablePath":        "string",
	"math/big.Int":  "json.Number",
	"time.Duration": "string",
	"os.FileMode":   "string",
	"github.com/multiformats/go-multicodec.Code":    "uint64",
	"github.com/multiformats/go-multibase.Encoding": "int",
	"time.Time": "string",
}

// reflectResponseType extracts TypeInfo for a command's response type.
// It returns nil if the type cannot be represented as a struct.
func reflectResponseType(goName string, t reflect.Type) []TypeInfo {
	if t == nil {
		return nil
	}

	// dereference pointer
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// for slices of structs, generate the element type
	if t.Kind() == reflect.Slice {
		elem := t.Elem()
		for elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		if elem.Kind() == reflect.Struct {
			types := reflectStruct(goName+"Item", elem, nil)
			// also generate a type alias for the slice
			return types
		}
		return nil
	}

	if t.Kind() == reflect.Chan {
		// channel types (like <-chan any) don't have useful structure
		return nil
	}

	if t.Kind() != reflect.Struct {
		return nil
	}

	return reflectStruct(goName+"Response", t, nil)
}

// reflectStruct extracts fields from a struct type, recursing into
// embedded/nested structs. Returns all generated types (main + nested).
func reflectStruct(name string, t reflect.Type, seen map[reflect.Type]bool) []TypeInfo {
	if seen == nil {
		seen = make(map[reflect.Type]bool)
	}
	if seen[t] {
		return nil
	}
	seen[t] = true

	var fields []FieldInfo
	var nested []TypeInfo

	for i := range t.NumField() {
		f := t.Field(i)

		// skip unexported fields (but process embedded)
		if !f.IsExported() && !f.Anonymous {
			continue
		}

		// embedded struct: flatten its fields
		if f.Anonymous {
			ft := f.Type
			for ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				embeddedTypes := reflectStruct(name, ft, seen)
				if len(embeddedTypes) > 0 {
					fields = append(fields, embeddedTypes[0].Fields...)
					nested = append(nested, embeddedTypes[1:]...)
				}
			}
			continue
		}

		jsonTag := jsonFieldTag(f)
		if jsonTag == "-" {
			continue
		}

		goType := resolveGoType(name, f.Name, f.Type, seen, &nested)
		fields = append(fields, FieldInfo{
			Name:    f.Name,
			Type:    goType,
			JSONTag: jsonTag,
		})
	}

	result := []TypeInfo{{Name: name, Fields: fields}}
	result = append(result, nested...)
	return result
}

// resolveGoType converts a reflect.Type to a Go type string for generated code.
func resolveGoType(parentName, fieldName string, t reflect.Type, seen map[reflect.Type]bool, nested *[]TypeInfo) string {
	// check known type mappings by full package path
	if t.PkgPath() != "" {
		fullKey := t.PkgPath() + "." + t.Name()
		if mapped, ok := knownFullPaths[fullKey]; ok {
			return mapped
		}
	}

	switch t.Kind() {
	case reflect.Ptr:
		inner := resolveGoType(parentName, fieldName, t.Elem(), seen, nested)
		return "*" + inner

	case reflect.Slice:
		elem := t.Elem()
		inner := resolveGoType(parentName, fieldName, elem, seen, nested)
		return "[]" + inner

	case reflect.Map:
		kType := resolveGoType(parentName, fieldName+"Key", t.Key(), seen, nested)
		vType := resolveGoType(parentName, fieldName+"Val", t.Elem(), seen, nested)
		return "map[" + kType + "]" + vType

	case reflect.Struct:
		nestedName := parentName + fieldName
		types := reflectStruct(nestedName, t, seen)
		if len(types) > 0 {
			*nested = append(*nested, types...)
			return types[0].Name
		}
		return "json.RawMessage"

	case reflect.Interface:
		return "json.RawMessage"

	case reflect.Bool:
		return "bool"
	case reflect.Int:
		return "int"
	case reflect.Int8:
		return "int8"
	case reflect.Int16:
		return "int16"
	case reflect.Int32:
		return "int32"
	case reflect.Int64:
		return "int64"
	case reflect.Uint:
		return "uint"
	case reflect.Uint8:
		return "uint8"
	case reflect.Uint16:
		return "uint16"
	case reflect.Uint32:
		return "uint32"
	case reflect.Uint64:
		return "uint64"
	case reflect.Float32:
		return "float32"
	case reflect.Float64:
		return "float64"
	case reflect.String:
		return "string"

	default:
		return "json.RawMessage"
	}
}

// typeName returns "pkg.Name" for a named type, or empty string.
func typeName(t reflect.Type) string {
	if t.PkgPath() == "" {
		return ""
	}
	pkg := t.PkgPath()
	if i := strings.LastIndex(pkg, "/"); i >= 0 {
		pkg = pkg[i+1:]
	}
	return pkg + "." + t.Name()
}

// jsonFieldTag extracts the JSON tag name for a struct field.
// Returns "FieldName,omitempty" if no tag is present.
func jsonFieldTag(f reflect.StructField) string {
	tag := f.Tag.Get("json")
	if tag == "" {
		return f.Name + ",omitempty"
	}
	return tag
}

// needsJSONImport checks if any type in the list uses json.RawMessage or json.Number.
func needsJSONImport(types []TypeInfo) bool {
	for _, t := range types {
		for _, f := range t.Fields {
			if strings.Contains(f.Type, "json.") {
				return true
			}
		}
	}
	return false
}

// deduplicateTypes removes duplicate TypeInfo entries by name, keeping the
// first occurrence.
func deduplicateTypes(types []TypeInfo) []TypeInfo {
	seen := make(map[string]bool)
	var result []TypeInfo
	for _, t := range types {
		if seen[t.Name] {
			continue
		}
		seen[t.Name] = true
		result = append(result, t)
	}
	return result
}

// sortTypes sorts types alphabetically by name.
func sortTypes(types []TypeInfo) {
	sort.Slice(types, func(i, j int) bool {
		return types[i].Name < types[j].Name
	})
}

// responseTypeName returns the Go type name for a command's response.
// For structs it returns "*GoNameResponse", for slices "[]GoNameItem".
func responseTypeName(cmd CommandInfo) string {
	if cmd.ResponseType == nil {
		return ""
	}

	t := cmd.ResponseType
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() == reflect.Slice {
		elem := t.Elem()
		for elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		if elem.Kind() == reflect.Struct {
			return fmt.Sprintf("[]%sItem", cmd.GoName)
		}
		return fmt.Sprintf("[]%s", resolveGoType("", "", elem, nil, nil))
	}

	if t.Kind() == reflect.Struct {
		return cmd.GoName + "Response"
	}

	return resolveGoType("", "", t, nil, nil)
}
