package main

import (
	"bytes"
	"fmt"
	"go/format"
	"reflect"
	"strings"
	"text/template"
	"unicode"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

// fileData holds all data needed to render one gen_*.go file.
type fileData struct {
	GroupName string
	Types     []TypeInfo
	Commands  []CommandInfo
	NeedsIO   bool
	NeedsJSON bool
	NeedsIter bool
}

// makeFuncMap returns the template function map with all real implementations.
func makeFuncMap() template.FuncMap {
	return template.FuncMap{
		"responseTypeName":     responseTypeName,
		"optionTypes":          renderOptionTypes,
		"methodFunc":           renderMethod,
		"argParams":            argParams,
		"argValues":            argValues,
		"returnType":           returnType,
		"streamReturnType":     streamReturnType,
		"streamItemType":       streamItemType,
		"streamZero":           streamZero,
		"hasStructResponse":    hasStructResponse,
		"hasSliceResponse":     hasSliceResponse,
		"fileBodySetup":        fileBodySetup,
		"toLower":              toLowerFirst,
		"sanitizeDescription":  sanitizeDescription,
		"hasPrimitiveResponse": hasPrimitiveResponse,
	}
}

// parseTmpl parses a template string with the real function map.
func parseTmpl(name, src string) *template.Template {
	return template.Must(template.New(name).Funcs(makeFuncMap()).Parse(src))
}

// generateFiles produces a map of filename -> content for all generated files.
func generateFiles(commands []CommandInfo) (map[string][]byte, error) {
	groups := groupCommands(commands)

	files := make(map[string][]byte)
	for groupName, cmds := range groups {
		content, err := generateGroupFile(groupName, cmds)
		if err != nil {
			return nil, fmt.Errorf("generating %s: %w", groupName, err)
		}
		fileName := fmt.Sprintf("gen_%s.go", groupName)
		files[fileName] = content
	}
	return files, nil
}

// groupCommands groups commands by their first path segment.
func groupCommands(commands []CommandInfo) map[string][]CommandInfo {
	groups := make(map[string][]CommandInfo)
	for _, cmd := range commands {
		groups[cmd.GroupName] = append(groups[cmd.GroupName], cmd)
	}
	return groups
}

// generateGroupFile generates one gen_*.go file for a command group.
func generateGroupFile(groupName string, commands []CommandInfo) ([]byte, error) {
	var allTypes []TypeInfo
	needsIO := false
	needsJSON := false
	needsIter := false

	for _, cmd := range commands {
		if cmd.ResponseType != nil {
			types := reflectResponseType(cmd.GoName, cmd.ResponseType)
			allTypes = append(allTypes, types...)
		}
		switch cmd.ResponseKind {
		case ResponseStream:
			needsJSON = true
			needsIter = true
		}
		if cmd.HasFileArg {
			needsIO = true
		}
	}

	allTypes = deduplicateTypes(allTypes)
	sortTypes(allTypes)

	if needsJSONImport(allTypes) {
		needsJSON = true
	}

	data := fileData{
		GroupName: groupName,
		Types:     allTypes,
		Commands:  commands,
		NeedsIO:   needsIO,
		NeedsJSON: needsJSON,
		NeedsIter: needsIter,
	}

	tmpl := parseTmpl("file", fileTemplateStr)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("formatting generated code for %s: %w\n--- raw source ---\n%s", groupName, err, buf.String())
	}
	return formatted, nil
}

// renderOptionTypes renders the option type and funcs for a command.
func renderOptionTypes(cmd CommandInfo) string {
	tmpl := parseTmpl("optTypes", optionTypesTemplateStr)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cmd); err != nil {
		panic(fmt.Sprintf("optionTypes template: %v", err))
	}
	return buf.String()
}

// renderMethod renders the method func for a command based on its ResponseKind.
func renderMethod(cmd CommandInfo) string {
	var tmplStr string

	switch cmd.ResponseKind {
	case ResponseStream:
		tmplStr = streamMethodTemplateStr
	case ResponseBinary:
		tmplStr = binaryMethodTemplateStr
	default: // ResponseSingle
		if cmd.ResponseType == nil {
			tmplStr = voidMethodTemplateStr
		} else if hasPrimitiveResponse(cmd) {
			// primitives, maps, interfaces - return *Response for manual decoding
			tmplStr = binaryMethodTemplateStr
		} else {
			tmplStr = singleMethodTemplateStr
		}
	}

	tmpl := parseTmpl("method", tmplStr)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, cmd); err != nil {
		panic(fmt.Sprintf("method template for %s: %v", cmd.Path, err))
	}
	return buf.String()
}

// argParams generates the function parameter list for positional arguments.
func argParams(cmd CommandInfo) string {
	var parts []string
	hasVariadic := false
	for _, arg := range cmd.Arguments {
		if arg.IsFile {
			parts = append(parts, fmt.Sprintf(", %s io.Reader", sanitizeArgName(arg.Name)))
		} else if arg.Variadic {
			hasVariadic = true
			// variadic args can't be in the middle, handle via options
		} else {
			parts = append(parts, fmt.Sprintf(", %s string", sanitizeArgName(arg.Name)))
		}
	}
	// if we have variadic string args AND options, the variadic must come
	// as a slice since Go only allows one variadic param at the end
	if hasVariadic && len(cmd.Options) > 0 {
		for _, arg := range cmd.Arguments {
			if arg.Variadic && !arg.IsFile {
				parts = append(parts, fmt.Sprintf(", %s []string", sanitizeArgName(arg.Name)))
			}
		}
	} else if hasVariadic {
		for _, arg := range cmd.Arguments {
			if arg.Variadic && !arg.IsFile {
				parts = append(parts, fmt.Sprintf(", %s ...string", sanitizeArgName(arg.Name)))
			}
		}
	}
	return strings.Join(parts, "")
}

// argValues generates the argument values passed to Request().
func argValues(cmd CommandInfo) string {
	var stringArgs []string
	hasVariadic := false

	for _, arg := range cmd.Arguments {
		if arg.IsFile {
			continue
		}
		if arg.Variadic {
			hasVariadic = true
			continue
		}
		stringArgs = append(stringArgs, sanitizeArgName(arg.Name))
	}

	result := ""
	if len(stringArgs) > 0 {
		result = ", " + strings.Join(stringArgs, ", ")
	}

	// handle variadic: append via .Arguments()
	if hasVariadic {
		for _, arg := range cmd.Arguments {
			if arg.Variadic && !arg.IsFile {
				name := sanitizeArgName(arg.Name)
				result += fmt.Sprintf(").Arguments(%s...", name)
			}
		}
	}

	return result
}

// returnType generates the return type for a single-response method.
func returnType(cmd CommandInfo) string {
	rt := responseTypeName(cmd)
	if rt == "" {
		return "error"
	}
	t := cmd.ResponseType
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice {
		return rt + ", error"
	}
	return "*" + rt + ", error"
}

// streamReturnType generates the return type for a streaming method.
func streamReturnType(cmd CommandInfo) string {
	item := streamItemType(cmd)
	return fmt.Sprintf("iter.Seq2[%s, error]", item)
}

// streamItemType returns the type of each streamed item.
func streamItemType(cmd CommandInfo) string {
	rt := responseTypeName(cmd)
	if rt == "" {
		return "json.RawMessage"
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
			return cmd.GoName + "Item"
		}
	}
	if t.Kind() == reflect.Struct {
		return cmd.GoName + "Response"
	}
	return "json.RawMessage"
}

// streamZero returns the zero value expression for the stream item type.
func streamZero(cmd CommandInfo) string {
	item := streamItemType(cmd)
	if item == "json.RawMessage" {
		return "nil"
	}
	return item + "{}"
}

// hasStructResponse returns true if the response is a struct (not slice).
func hasStructResponse(cmd CommandInfo) bool {
	if cmd.ResponseType == nil {
		return false
	}
	t := cmd.ResponseType
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct
}

// hasSliceResponse returns true if the response is a slice.
func hasSliceResponse(cmd CommandInfo) bool {
	if cmd.ResponseType == nil {
		return false
	}
	t := cmd.ResponseType
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Kind() == reflect.Slice
}

// hasPrimitiveResponse returns true if the response is a primitive, map, or other non-struct/non-slice type.
func hasPrimitiveResponse(cmd CommandInfo) bool {
	if cmd.ResponseType == nil {
		return false
	}
	t := cmd.ResponseType
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Kind() != reflect.Struct && t.Kind() != reflect.Slice
}

// fileBodySetup generates the FileBody call for file arguments.
func fileBodySetup(cmd CommandInfo) string {
	for _, arg := range cmd.Arguments {
		if arg.IsFile {
			return fmt.Sprintf("req = req.FileBody(%s).(RequestBuilder)", sanitizeArgName(arg.Name))
		}
	}
	return ""
}

// sanitizeDescription cleans a description string for use in Go comments.
// Replaces newlines with spaces and trims trailing whitespace.
func sanitizeDescription(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	return strings.TrimSpace(s)
}

// sanitizeArgName converts argument names to valid Go identifiers.
func sanitizeArgName(name string) string {
	name = strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(name)
	switch name {
	case "type", "func", "var", "const", "map", "range", "chan", "select", "default", "interface":
		return name + "Arg"
	}
	return name
}

// toLowerFirst lowercases the first letter of a string.
func toLowerFirst(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// statusComment returns a deprecation/experimental comment prefix.
func statusComment(s cmds.Status) string {
	switch s {
	case cmds.Deprecated:
		return "Deprecated"
	case cmds.Experimental:
		return "Experimental"
	default:
		return ""
	}
}
