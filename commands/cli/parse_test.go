package cli

import (
	"strings"
	"testing"

	"github.com/ipfs/go-ipfs/commands"
)

func TestOptionParsing(t *testing.T) {
	subCmd := &commands.Command{}
	cmd := &commands.Command{
		Options: []commands.Option{
			commands.StringOption("string", "s", "a string"),
			commands.BoolOption("bool", "b", "a bool"),
		},
		Subcommands: map[string]*commands.Command{
			"test": subCmd,
		},
	}

	type kvs map[string]interface{}
	type words []string

	sameWords := func(a words, b words) bool {
		for i, w := range a {
			if w != b[i] {
				return false
			}
		}
		return true
	}

	sameKVs := func(a kvs, b kvs) bool {
		if len(a) != len(b) {
			return false
		}
		for k, v := range a {
			if v != b[k] {
				return false
			}
		}
		return true
	}

	testHelper := func(args string, expectedOpts kvs, expectedWords words, expectErr bool) {
		_, opts, input, _, err := parseOpts(strings.Split(args, " "), cmd)
		if expectErr {
			if err == nil {
				t.Errorf("Command line '%v' parsing should have failed", args)
			}
		} else if err != nil {
			t.Errorf("Command line '%v' failed to parse: %v", args, err)
		} else if !sameWords(input, expectedWords) || !sameKVs(opts, expectedOpts) {
			t.Errorf("Command line '%v':\n  parsed as  %v %v\n  instead of %v %v",
				args, opts, input, expectedOpts, expectedWords)
		}
	}

	testFail := func(args string) {
		testHelper(args, kvs{}, words{}, true)
	}

	test := func(args string, expectedOpts kvs, expectedWords words) {
		testHelper(args, expectedOpts, expectedWords, false)
	}

	test("-", kvs{}, words{"-"})
	testFail("-b -b")
	test("beep boop", kvs{}, words{"beep", "boop"})
	test("test beep boop", kvs{}, words{"beep", "boop"})
	testFail("-s")
	test("-s foo", kvs{"s": "foo"}, words{})
	test("-sfoo", kvs{"s": "foo"}, words{})
	test("-s=foo", kvs{"s": "foo"}, words{})
	test("-b", kvs{"b": ""}, words{})
	test("-bs foo", kvs{"b": "", "s": "foo"}, words{})
	test("-sb", kvs{"s": "b"}, words{})
	test("-b foo", kvs{"b": ""}, words{"foo"})
	test("--bool foo", kvs{"bool": ""}, words{"foo"})
	testFail("--bool=foo")
	testFail("--string")
	test("--string foo", kvs{"string": "foo"}, words{})
	test("--string=foo", kvs{"string": "foo"}, words{})
	test("-- -b", kvs{}, words{"-b"})
	test("foo -b", kvs{"b": ""}, words{"foo"})
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
