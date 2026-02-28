package main

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"unicode"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/kubo/core/commands/cmdutils"
)

// CommandInfo holds the extracted metadata for one HTTP-accessible command.
type CommandInfo struct {
	Path         string       // e.g. "pin/add"
	GoName       string       // e.g. "PinAdd"
	GroupName    string       // e.g. "pin"
	GroupGoName  string       // e.g. "Pin"
	Arguments    []ArgInfo    // positional args
	Options      []OptInfo    // command options (excludes global opts)
	ResponseKind ResponseKind // how to consume the HTTP response
	ResponseType reflect.Type // nil for Binary / void
	HasFileArg   bool         // true if any argument is ArgFile
	Status       cmds.Status
	Tagline      string
}

// ResponseKind mirrors cmdutils.ResponseKind.
type ResponseKind = cmdutils.ResponseKind

const (
	ResponseSingle = cmdutils.ResponseSingle
	ResponseStream = cmdutils.ResponseStream
	ResponseBinary = cmdutils.ResponseBinary
)

// ArgInfo describes a positional argument.
type ArgInfo struct {
	Name     string
	IsFile   bool // ArgFile vs ArgString
	Required bool
	Variadic bool
}

// OptInfo describes a command option.
type OptInfo struct {
	Name        string // primary name (first in Names())
	GoName      string // PascalCase
	Type        string // Go type: "bool", "int", "int64", "uint64", "float64", "string", "[]string"
	Default     any
	Description string
}

// walkCommandTree recursively collects CommandInfo for all HTTP-accessible
// commands under root. The prefix is prepended to the command path (pass ""
// for the root).
func walkCommandTree(root *cmds.Command, prefix string) []CommandInfo {
	var result []CommandInfo

	for name, sub := range root.Subcommands {
		path := name
		if prefix != "" {
			path = prefix + "/" + name
		}

		// skip commands that can't be called over HTTP
		if sub.Run != nil && !sub.NoRemote && sub.Status != cmds.Removed {
			info := extractCommandInfo(path, sub)
			result = append(result, info)
		}

		// always recurse into subcommands
		result = append(result, walkCommandTree(sub, path)...)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})
	return result
}

// extractCommandInfo builds a CommandInfo from a command definition.
func extractCommandInfo(path string, cmd *cmds.Command) CommandInfo {
	info := CommandInfo{
		Path:         path,
		GoName:       pathToGoName(path),
		GroupName:    groupName(path),
		GroupGoName:  toPascalCase(groupName(path)),
		ResponseKind: cmdutils.GetResponseKind(cmd),
		Status:       cmd.Status,
		Tagline:      cmd.Helptext.Tagline,
	}

	if cmd.Type != nil {
		info.ResponseType = reflect.TypeOf(cmd.Type)
		if info.ResponseType.Kind() == reflect.Ptr {
			info.ResponseType = info.ResponseType.Elem()
		}
	}

	// arguments
	for _, arg := range cmd.Arguments {
		a := ArgInfo{
			Name:     arg.Name,
			IsFile:   arg.Type == cmds.ArgFile,
			Required: arg.Required,
			Variadic: arg.Variadic,
		}
		if a.IsFile {
			info.HasFileArg = true
		}
		info.Arguments = append(info.Arguments, a)
	}

	// options (skip global options that come from the framework)
	for _, opt := range cmd.Options {
		name := opt.Names()[0]
		if isGlobalOption(name) {
			continue
		}
		info.Options = append(info.Options, OptInfo{
			Name:        name,
			GoName:      toPascalCase(name),
			Type:        optionGoType(opt),
			Default:     opt.Default(),
			Description: opt.Description(),
		})
	}

	return info
}

// globalOptions lists option names provided by the framework or the root
// command that every command inherits. We skip these in generated code.
var globalOptions = map[string]struct{}{
	"encoding":                {},
	"stream-channels":         {},
	"timeout":                 {},
	"api":                     {},
	"api-auth":                {},
	"offline":                 {},
	"repo-dir":                {},
	"config-file":             {},
	"config":                  {},
	"debug":                   {},
	"help":                    {},
	"local":                   {},
	"cid-base":                {},
	"upgrade-cidv0-in-output": {},
}

func isGlobalOption(name string) bool {
	_, ok := globalOptions[name]
	return ok
}

// optionGoType returns the Go type string for a cmds.Option.
func optionGoType(opt cmds.Option) string {
	switch opt.Type() {
	case reflect.Bool:
		return "bool"
	case reflect.Int:
		return "int"
	case reflect.Int64:
		return "int64"
	case reflect.Uint:
		return "uint"
	case reflect.Uint64:
		return "uint64"
	case reflect.Float64:
		return "float64"
	case reflect.String:
		return "string"
	case reflect.Array, reflect.Slice:
		return "[]string"
	default:
		return "string"
	}
}

// pathToGoName converts "pin/add" to "PinAdd".
func pathToGoName(path string) string {
	parts := strings.Split(path, "/")
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(toPascalCase(p))
	}
	return b.String()
}

// toPascalCase converts "find-provs" to "FindProvs", "bw" to "Bw", etc.
func toPascalCase(s string) string {
	var b strings.Builder
	upper := true
	for _, r := range s {
		if r == '-' || r == '_' {
			upper = true
			continue
		}
		if upper {
			b.WriteRune(unicode.ToUpper(r))
			upper = false
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// groupName returns the first path segment.
func groupName(path string) string {
	if i := strings.IndexByte(path, '/'); i >= 0 {
		return path[:i]
	}
	return path
}

func formatDefault(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
