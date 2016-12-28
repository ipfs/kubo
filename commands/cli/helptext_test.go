package cli

import (
	"strings"
	"testing"

	"bytes"
	cmds "github.com/ipfs/go-ipfs/commands"
	"github.com/ipfs/go-ipfs/core/commands"
)

func TestSynopsisGenerator(t *testing.T) {
	command := &cmds.Command{
		Arguments: []cmds.Argument{
			cmds.StringArg("required", true, false, ""),
			cmds.StringArg("variadic", false, true, ""),
		},
		Options: []cmds.Option{
			cmds.StringOption("opt", "o", "Option"),
		},
		Helptext: cmds.HelpText{
			SynopsisOptionsValues: map[string]string{
				"opt": "OPTION",
			},
		},
	}
	syn := generateSynopsis(command, "cmd")
	t.Logf("Synopsis is: %s", syn)
	if !strings.HasPrefix(syn, "cmd ") {
		t.Fatal("Synopsis should start with command name")
	}
	if !strings.Contains(syn, "[--opt=<OPTION> | -o]") {
		t.Fatal("Synopsis should contain option descriptor")
	}
	if !strings.Contains(syn, "<required>") {
		t.Fatal("Synopsis should contain required argument")
	}
	if !strings.Contains(syn, "<variadic>...") {
		t.Fatal("Synopsis should contain variadic argument")
	}
	if !strings.Contains(syn, "[<variadic>...]") {
		t.Fatal("Synopsis should contain optional argument")
	}
	if !strings.Contains(syn, "[--]") {
		t.Fatal("Synopsis should contain options finalizer")
	}
}

func TestColorOutput(t *testing.T) {
	opt := cmds.BoolOption("color", "Use colors in console output.").Default(false)
	opts := map[string]cmds.Option{"color": opt}
	colorReq, _ := cmds.NewRequest(nil, cmds.OptMap{"color": true}, []string{}, nil, nil, opts)
	buf := new(bytes.Buffer)

	LongHelp("ipfs", commands.Root, []string{}, colorReq, buf)
	colorFullLongHelp := buf.String()
	cyanCount := strings.Count(colorFullLongHelp, formatCyan)
	resetCount := strings.Count(colorFullLongHelp, formatReset)
	if cyanCount == 0 {
		t.Fatal("Colorful long help should contain the cyan escape code")
	}
	if resetCount == 0 {
		t.Fatal("Colorful long help should contain the reset escape code")
	}
	if resetCount < cyanCount {
		t.Fatal("There should be at least as many reset escape codes as cyan escape codes")
	}

	buf = new(bytes.Buffer)
	ShortHelp("ipfs", commands.Root, []string{}, colorReq, buf)
	colorFullShortHelp := buf.String()
	cyanCount = strings.Count(colorFullShortHelp, formatCyan)
	resetCount = strings.Count(colorFullShortHelp, formatReset)
	if cyanCount == 0 {
		t.Fatal("Colorful shot help should contain the cyan escape code")
	}
	if resetCount == 0 {
		t.Fatal("Colorful shot help should contain the reset escape code")
	}
	if resetCount < cyanCount {
		t.Fatal("There should be at least as many reset escape codes as cyan escape codes")
	}

}
