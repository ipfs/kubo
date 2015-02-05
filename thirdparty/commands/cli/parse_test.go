package cli

import (
	//"fmt"
	"testing"

	"github.com/jbenet/go-ipfs/thirdparty/commands"
)

func TestOptionParsing(t *testing.T) {
	subCmd := &commands.Command{}
	cmd := &commands.Command{
		Options: []commands.Option{
			commands.StringOption("b", "some option"),
		},
		Subcommands: map[string]*commands.Command{
			"test": subCmd,
		},
	}

	opts, input, err := parseOptions([]string{"--beep", "-boop=lol", "test2", "-c", "beep", "--foo=5"})
	/*for k, v := range opts {
	    fmt.Printf("%s: %s\n", k, v)
	  }
	  fmt.Printf("%s\n", input)*/
	if err != nil {
		t.Error("Should have passed")
	}
	if len(opts) != 4 || opts["beep"] != "" || opts["boop"] != "lol" || opts["c"] != "" || opts["foo"] != "5" {
		t.Errorf("Returned options were defferent than expected: %v", opts)
	}
	if len(input) != 2 || input[0] != "test2" || input[1] != "beep" {
		t.Errorf("Returned input was different than expected: %v", input)
	}

	_, _, err = parseOptions([]string{"-beep=1", "-boop=2", "-beep=3"})
	if err == nil {
		t.Error("Should have failed (duplicate option name)")
	}

	path, args, sub := parsePath([]string{"test", "beep", "boop"}, cmd)
	if len(path) != 1 || path[0] != "test" {
		t.Errorf("Returned path was defferent than expected: %v", path)
	}
	if len(args) != 2 || args[0] != "beep" || args[1] != "boop" {
		t.Errorf("Returned args were different than expected: %v", args)
	}
	if sub != subCmd {
		t.Errorf("Returned command was different than expected")
	}
}

func TestArgumentParsing(t *testing.T) {
	rootCmd := &commands.Command{
		Subcommands: map[string]*commands.Command{
			"noarg": &commands.Command{},
			"onearg": &commands.Command{
				Arguments: []commands.Argument{
					commands.StringArg("a", true, false, "some arg"),
				},
			},
			"twoargs": &commands.Command{
				Arguments: []commands.Argument{
					commands.StringArg("a", true, false, "some arg"),
					commands.StringArg("b", true, false, "another arg"),
				},
			},
			"variadic": &commands.Command{
				Arguments: []commands.Argument{
					commands.StringArg("a", true, true, "some arg"),
				},
			},
			"optional": &commands.Command{
				Arguments: []commands.Argument{
					commands.StringArg("b", false, true, "another arg"),
				},
			},
			"reversedoptional": &commands.Command{
				Arguments: []commands.Argument{
					commands.StringArg("a", false, false, "some arg"),
					commands.StringArg("b", true, false, "another arg"),
				},
			},
		},
	}

	_, _, _, err := Parse([]string{"noarg"}, nil, rootCmd)
	if err != nil {
		t.Error("Should have passed")
	}
	_, _, _, err = Parse([]string{"noarg", "value!"}, nil, rootCmd)
	if err == nil {
		t.Error("Should have failed (provided an arg, but command didn't define any)")
	}

	_, _, _, err = Parse([]string{"onearg", "value!"}, nil, rootCmd)
	if err != nil {
		t.Error("Should have passed")
	}
	_, _, _, err = Parse([]string{"onearg"}, nil, rootCmd)
	if err == nil {
		t.Error("Should have failed (didn't provide any args, arg is required)")
	}

	_, _, _, err = Parse([]string{"twoargs", "value1", "value2"}, nil, rootCmd)
	if err != nil {
		t.Error("Should have passed")
	}
	_, _, _, err = Parse([]string{"twoargs", "value!"}, nil, rootCmd)
	if err == nil {
		t.Error("Should have failed (only provided 1 arg, needs 2)")
	}
	_, _, _, err = Parse([]string{"twoargs"}, nil, rootCmd)
	if err == nil {
		t.Error("Should have failed (didn't provide any args, 2 required)")
	}

	_, _, _, err = Parse([]string{"variadic", "value!"}, nil, rootCmd)
	if err != nil {
		t.Error("Should have passed")
	}
	_, _, _, err = Parse([]string{"variadic", "value1", "value2", "value3"}, nil, rootCmd)
	if err != nil {
		t.Error("Should have passed")
	}
	_, _, _, err = Parse([]string{"variadic"}, nil, rootCmd)
	if err == nil {
		t.Error("Should have failed (didn't provide any args, 1 required)")
	}

	_, _, _, err = Parse([]string{"optional", "value!"}, nil, rootCmd)
	if err != nil {
		t.Error("Should have passed")
	}
	_, _, _, err = Parse([]string{"optional"}, nil, rootCmd)
	if err != nil {
		t.Error("Should have passed")
	}

	_, _, _, err = Parse([]string{"reversedoptional", "value1", "value2"}, nil, rootCmd)
	if err != nil {
		t.Error("Should have passed")
	}
	_, _, _, err = Parse([]string{"reversedoptional", "value!"}, nil, rootCmd)
	if err != nil {
		t.Error("Should have passed")
	}
	_, _, _, err = Parse([]string{"reversedoptional"}, nil, rootCmd)
	if err == nil {
		t.Error("Should have failed (didn't provide any args, 1 required)")
	}
	_, _, _, err = Parse([]string{"reversedoptional", "value1", "value2", "value3"}, nil, rootCmd)
	if err == nil {
		t.Error("Should have failed (provided too many args, only takes 1)")
	}
}
