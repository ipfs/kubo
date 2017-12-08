package commands

import (
	"strings"
	"testing"

	"gx/ipfs/QmUyfy4QSr3NXym4etEiRyxBLqqAeKHJuRdi8AACxg63fZ/go-ipfs-cmdkit"
	"gx/ipfs/QmamUWYjFeYYzFDFPTvnmGkozJigsoDWUA4zoifTRFTnwK/go-ipfs-cmds"
)

func TestGetOutputPath(t *testing.T) {
	cases := []struct {
		args    []string
		opts    cmdkit.OptMap
		outPath string
	}{
		{
			args: []string{"/ipns/multiformats.io/"},
			opts: cmdkit.OptMap{
				"output": "takes-precedence",
			},
			outPath: "takes-precedence",
		},
		{
			args:    []string{"/ipns/multiformats.io/"},
			outPath: "multiformats.io",
			opts:    cmdkit.OptMap{},
		},
		{
			args:    []string{"/ipns/multiformats.io/logo.svg/"},
			outPath: "logo.svg",
			opts:    cmdkit.OptMap{},
		},
		{
			args:    []string{"/ipns/multiformats.io"},
			outPath: "multiformats.io",
			opts:    cmdkit.OptMap{},
		},
	}

	for _, tc := range cases {
		req := getGetCmdReq(tc.args, nil, t)

		if outPath := getOutPath(req); outPath != tc.outPath {
			t.Errorf("expected outPath %s to be %s", outPath, tc.outPath)
		}
	}
}

func TestGetCmdPathCleanupInPreRun(t *testing.T) {
	cases := []struct {
		args     []string
		wantArgs []string
	}{
		{
			args:     []string{"/ipns/multiformats.io/"},
			wantArgs: []string{"/ipns/multiformats.io"},
		},
		{
			args:     []string{"/ipns/multiformats.io"},
			wantArgs: []string{"/ipns/multiformats.io"},
		},
		{
			args:     []string{"/ipns/multiformats.io/logo.svg/"},
			wantArgs: []string{"/ipns/multiformats.io/logo.svg"},
		},
	}

	for _, tc := range cases {
		req := getGetCmdReq(tc.args, nil, t)

		err := GetCmd.PreRun(req)
		if err != nil {
			t.Fatalf("get command PreRun failed with error: %v", err)
		}
		if pathArg := req.Arguments()[0]; strings.HasSuffix(pathArg, "/") {
			t.Errorf("trailing suffix should have been removed, got %s", pathArg)
		}
	}
}

func getGetCmdReq(args []string, opts cmdkit.OptMap, t *testing.T) cmds.Request {
	defOpts, err := GetCmd.GetOptions([]string{})
	if err != nil {
		t.Fatalf("error getting default command options: %v", err)
	}

	req, err := cmds.NewRequest([]string{}, opts, args, nil, GetCmd, defOpts)
	if err != nil {
		t.Fatalf("error creating a command request: %v", err)
	}
	return req
}
