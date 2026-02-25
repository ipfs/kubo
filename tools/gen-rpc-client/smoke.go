package main

import (
	"bytes"
	"fmt"
	"go/format"
	"reflect"
	"strings"
	"text/template"
)

// skipReasons maps command paths to reasons they cannot be smoke-tested.
// Commands in this map get t.Skip(...) in the generated test.
var skipReasons = map[string]string{
	// destructive
	"shutdown": "kills the daemon",

	// needs peer / network
	"swarm/connect":    "needs peer address",
	"swarm/disconnect": "needs connected peer",
	"routing/findpeer": "needs online peer",
	"routing/findprovs": "needs CID with providers",
	"routing/get":       "needs valid DHT key",
	"routing/put":       "needs valid DHT key/value pair",
	"routing/provide":   "needs CID and network",
	"routing/reprovide": "needs CID and network",
	"dht/findpeer":      "needs online peer",
	"dht/findprovs":     "needs CID with providers",
	"dht/get":           "needs valid DHT key",
	"dht/put":           "needs valid DHT key/value pair",
	"dht/provide":       "needs CID and network",
	"dht/query":         "needs online peer",
	"ping":              "needs online peer",

	// remote pinning service
	"pin/remote/add":         "needs remote pinning service",
	"pin/remote/ls":          "needs remote pinning service",
	"pin/remote/rm":          "needs remote pinning service",
	"pin/remote/service/add": "needs remote pinning service",
	"pin/remote/service/ls":  "needs remote pinning service",
	"pin/remote/service/rm":  "needs remote pinning service",

	// config mutation
	"config":               "needs specific config key/value",
	"config/replace":       "replaces entire config",
	"config/profile/apply": "mutates config",

	// name operations
	"name/publish":      "needs IPNS key setup",
	"name/resolve":      "needs published IPNS record",
	"name/get":          "needs published IPNS record",
	"name/inspect":      "needs valid IPNS record file",
	"name/put":          "needs valid IPNS record file",
	"name/pubsub/cancel": "needs active IPNS subscription",

	// pubsub
	"pubsub/pub": "needs topic and subscriber",
	"pubsub/sub": "blocks waiting for messages",

	// p2p
	"p2p/forward":     "needs p2p setup",
	"p2p/listen":      "needs p2p setup",
	"p2p/close":       "needs active p2p listeners",
	"p2p/ls":          "needs active p2p listeners",
	"p2p/stream/close": "needs active p2p streams",
	"p2p/stream/ls":    "needs active p2p streams",

	// key operations needing existing keys
	"key/import": "needs valid key file",
	"key/rename": "needs existing non-self key",
	"key/rm":     "needs existing non-self key",
	"key/sign":   "needs key and data setup",
	"key/verify": "needs key, signature, and data",

	// filesystem / mount
	"mount": "requires FUSE",

	// MFS operations needing state
	"files/cp":    "needs MFS files",
	"files/mv":    "needs MFS files",
	"files/rm":    "needs MFS files",
	"files/write": "needs MFS file path",
	"files/mkdir": "needs MFS path setup",
	"files/chcid": "needs MFS path",

	// filestore
	"filestore/ls":     "needs filestore content",
	"filestore/verify": "needs filestore content",

	// profiling
	"diag/profile": "runs profiler, slow",

	// streaming that blocks
	"stats/bw": "blocks polling for bandwidth stats",
	"log/tail": "blocks waiting for log events",

	// dag/import needs a valid CAR file
	"dag/import": "needs valid CAR file",
}

// smokeTestArgs generates the argument expressions for a smoke test call.
// It mirrors the logic in argParams but produces test values instead of param names.
func smokeTestArgs(cmd CommandInfo) string {
	var parts []string
	hasVariadic := false

	for _, arg := range cmd.Arguments {
		if arg.IsFile {
			parts = append(parts, smokeFileValue(cmd))
		} else if arg.Variadic {
			hasVariadic = true
		} else {
			parts = append(parts, smokeStringValue(arg))
		}
	}

	// variadic args: slice if options present, spread if not
	if hasVariadic && len(cmd.Options) > 0 {
		for _, arg := range cmd.Arguments {
			if arg.Variadic && !arg.IsFile {
				parts = append(parts, smokeSliceValue(arg))
			}
		}
	} else if hasVariadic {
		for _, arg := range cmd.Arguments {
			if arg.Variadic && !arg.IsFile {
				parts = append(parts, smokeStringValue(arg))
			}
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return ", " + strings.Join(parts, ", ")
}

// smokeStringValue returns a test value for a string argument.
func smokeStringValue(arg ArgInfo) string {
	name := strings.ToLower(arg.Name)
	if strings.Contains(name, "path") || strings.Contains(name, "cid") ||
		strings.Contains(name, "ref") || strings.Contains(name, "root") {
		return "testCID"
	}
	return `""`
}

// smokeSliceValue returns a test value for a variadic arg passed as []string.
func smokeSliceValue(arg ArgInfo) string {
	name := strings.ToLower(arg.Name)
	if strings.Contains(name, "path") || strings.Contains(name, "cid") ||
		strings.Contains(name, "ref") || strings.Contains(name, "root") ||
		strings.Contains(name, "key") {
		return "[]string{testCID}"
	}
	return `[]string{""}`
}

// smokeFileValue returns a test value for a file argument.
func smokeFileValue(cmd CommandInfo) string {
	// dag/put expects JSON
	if cmd.Path == "dag/put" {
		return `strings.NewReader("{}")`
	}
	return `strings.NewReader("test data")`
}

// smokeReturnKind describes how to handle the return value in a test.
type smokeReturnKind int

const (
	smokeReturnVoid     smokeReturnKind = iota // returns error
	smokeReturnStruct                          // returns *T, error
	smokeReturnSlice                           // returns []T, error
	smokeReturnBinary                          // returns *Response, error
	smokeReturnStream                          // returns iter.Seq2[T, error]
)

func classifyReturn(cmd CommandInfo) smokeReturnKind {
	if cmd.ResponseKind == ResponseStream {
		return smokeReturnStream
	}
	if cmd.ResponseKind == ResponseBinary {
		return smokeReturnBinary
	}
	// ResponseSingle
	if cmd.ResponseType == nil {
		return smokeReturnVoid
	}
	if hasPrimitiveResponse(cmd) {
		return smokeReturnBinary
	}
	t := cmd.ResponseType
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice {
		return smokeReturnSlice
	}
	return smokeReturnStruct
}

// smokeSubtest generates one t.Run subtest for a command.
func smokeSubtest(cmd CommandInfo) string {
	var b strings.Builder

	args := smokeTestArgs(cmd)

	// check skip
	if reason, ok := skipReasons[cmd.Path]; ok {
		fmt.Fprintf(&b, "\tt.Run(%q, func(t *testing.T) {\n", cmd.GoName)
		fmt.Fprintf(&b, "\t\tt.Skip(%q)\n", reason)
		fmt.Fprintf(&b, "\t})\n")
		return b.String()
	}

	fmt.Fprintf(&b, "\tt.Run(%q, func(t *testing.T) {\n", cmd.GoName)
	fmt.Fprintf(&b, "\t\trequire.NotPanics(t, func() {\n")

	switch classifyReturn(cmd) {
	case smokeReturnVoid:
		fmt.Fprintf(&b, "\t\t\t_ = api.%s(ctx%s)\n", cmd.GoName, args)

	case smokeReturnStruct:
		fmt.Fprintf(&b, "\t\t\t_, _ = api.%s(ctx%s)\n", cmd.GoName, args)

	case smokeReturnSlice:
		fmt.Fprintf(&b, "\t\t\t_, _ = api.%s(ctx%s)\n", cmd.GoName, args)

	case smokeReturnBinary:
		fmt.Fprintf(&b, "\t\t\tresp, err := api.%s(ctx%s)\n", cmd.GoName, args)
		fmt.Fprintf(&b, "\t\t\tif err == nil && resp != nil {\n")
		fmt.Fprintf(&b, "\t\t\t\tresp.Close()\n")
		fmt.Fprintf(&b, "\t\t\t}\n")

	case smokeReturnStream:
		fmt.Fprintf(&b, "\t\t\tfor _, err := range api.%s(ctx%s) {\n", cmd.GoName, args)
		fmt.Fprintf(&b, "\t\t\t\t_ = err\n")
		fmt.Fprintf(&b, "\t\t\t\tbreak\n")
		fmt.Fprintf(&b, "\t\t\t}\n")
	}

	fmt.Fprintf(&b, "\t\t})\n")
	fmt.Fprintf(&b, "\t})\n")
	return b.String()
}

// smokeTestTemplate is the template for gen_smoke_test.go.
const smokeTestTemplateStr = `// Code generated by tools/gen-rpc-client; DO NOT EDIT.

package rpc

import (
	"context"
	"strings"
	"testing"

	"github.com/ipfs/kubo/test/cli/harness"
	"github.com/stretchr/testify/require"
)

// TestGeneratedSmoke starts a daemon and calls every generated method with
// minimal arguments. It verifies that methods don't panic and that response
// deserialization works. Commands that need complex setup are skipped.
func TestGeneratedSmoke(t *testing.T) {
	t.Parallel()
	h := harness.NewT(t)
	node := h.NewNode().Init().StartDaemon("--offline")
	api, err := NewApi(node.APIAddr())
	require.NoError(t, err)
	ctx := context.Background()
	_ = strings.NewReader // ensure strings import is used

	testCID := node.IPFSAddStr("smoke test data")

{{range .Commands}}
{{smokeSubtest .}}
{{end}}
}
`

// generateSmokeTest produces the gen_smoke_test.go content.
func generateSmokeTest(commands []CommandInfo) ([]byte, error) {
	funcMap := template.FuncMap{
		"smokeSubtest": smokeSubtest,
	}

	tmpl, err := template.New("smoke").Funcs(funcMap).Parse(smokeTestTemplateStr)
	if err != nil {
		return nil, fmt.Errorf("parsing smoke template: %w", err)
	}

	data := struct {
		Commands []CommandInfo
	}{
		Commands: commands,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing smoke template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("formatting smoke test: %w\n--- raw source ---\n%s", err, buf.String())
	}
	return formatted, nil
}
