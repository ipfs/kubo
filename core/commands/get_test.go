package commands

import (
	"testing"

	"gx/ipfs/QmTD5GyjsmygZ5mzyD28x3QRTYL9yeNcSmcQjnZ93g1NuY/go-ipfs-cmds"
	"gx/ipfs/QmUyfy4QSr3NXym4etEiRyxBLqqAeKHJuRdi8AACxg63fZ/go-ipfs-cmdkit"
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

	defOpts, err := GetCmd.GetOptions([]string{})
	if err != nil {
		t.Fatalf("error getting default command options: %v", err)
	}

	for _, tc := range cases {
		req, err := cmds.NewRequest([]string{}, tc.opts, tc.args, nil, GetCmd, defOpts)
		if err != nil {
			t.Fatalf("error creating a command request: %v", err)
		}
		if outPath := getOutPath(req); outPath != tc.outPath {
			t.Errorf("expected outPath %s to be %s", outPath, tc.outPath)
		}
	}
}
