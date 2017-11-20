package commands

import (
	"context"
	"testing"

	cmds "gx/ipfs/QmTwKPLyeRKuDawuy6CAn1kRj1FVoqBEM8sviAUWN7NW9K/go-ipfs-cmds"
	cmdkit "gx/ipfs/QmVD1W3MC8Hk1WZgFQPWWmBECJ3X72BgUYf9eCQ4PGzPps/go-ipfs-cmdkit"
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
