package commands

import (
	"context"
	"testing"

	cmdkit "gx/ipfs/QmSRaAPPNxyhnXeDa5NXtZ2CWBYJ6BRWNQp6gKxhPcoqDM/go-ipfs-cmdkit"
	cmds "gx/ipfs/QmUjA7onJjYmZ2TivD83MeCfXf5o1U4ByVJWPjRru5NA4t/go-ipfs-cmds"
)

func TestGetOutputPath(t *testing.T) {
	cases := []struct {
		args    []string
		opts    cmdkit.OptMap
		outPath string
	}{
		{
			args: []string{"/ipns/multiformats.io/"},
			opts: map[string]interface{}{
				"output": "takes-precedence",
			},
			outPath: "takes-precedence",
		},
		{
			args: []string{"/ipns/multiformats.io/", "some-other-arg-to-be-ignored"},
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
			args:    []string{"/ipns/multiformats.io", "some-other-arg-to-be-ignored"},
			outPath: "multiformats.io",
			opts:    cmdkit.OptMap{},
		},
	}

	/*
		defOpts, err := GetCmd.GetOptions([]string{})
		if err != nil {
			t.Fatalf("error getting default command options: %v", err)
		}
	*/

	for _, tc := range cases {
		req, err := cmds.NewRequest(context.TODO(), []string{}, tc.opts, tc.args, nil, GetCmd)
		if err != nil {
			t.Fatalf("error creating a command request: %v", err)
		}
		if outPath := getOutPath(req); outPath != tc.outPath {
			t.Errorf("expected outPath %s to be %s", outPath, tc.outPath)
		}
	}
}
