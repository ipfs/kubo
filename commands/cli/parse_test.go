package cli

import (
	"strings"
	"testing"
	"io"
	"io/ioutil"
	"os"

	"github.com/ipfs/go-ipfs/commands"
)

type kvs map[string]interface{}
type words []string

func sameWords(a words, b words) bool {
	if len(a) != len(b) {
		return false
	}
	for i, w := range a {
		if w != b[i] {
			return false
		}
	}
	return true
}

func sameKVs(a kvs, b kvs) bool {
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

func TestSameWords(t *testing.T) {
	a := []string{"v1", "v2"}
	b := []string{"v1", "v2", "v3"}
	c := []string{"v2", "v3"}
	d := []string{"v2"}
	e := []string{"v2", "v3"}
	f := []string{"v2", "v1"}

	test := func(a words, b words, v bool) {
		if sameWords(a, b) != v {
			t.Errorf("sameWords('%v', '%v') != %v", a, b, v)
		}
	}

	test(a, b, false)
	test(a, a, true)
	test(a, c, false)
	test(b, c, false)
	test(c, d, false)
	test(c, e, true)
	test(b, e, false)
	test(a, b, false)
	test(a, f, false)
	test(e, f, false)
	test(f, f, true)
}

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
			"stdinenabled": &commands.Command{
				Arguments: []commands.Argument{
					commands.StringArg("a", true, true, "some arg").EnableStdin(),
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

	// Use a temp file to simulate stdin
	fileToSimulateStdin := func(t *testing.T, content string) (*os.File) {
		fstdin, err := ioutil.TempFile("", "")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(fstdin.Name())

		if _, err := io.WriteString(fstdin, content); err != nil {
			t.Fatal(err)
		}
		return fstdin
	}

	test := func(cmd words, f *os.File, res words) {
		if f != nil {
			if _, err := f.Seek(0, os.SEEK_SET); err != nil {
				t.Fatal(err)
			}
		}
		req, _, _, err := Parse(cmd, f, rootCmd)
		if err != nil {
			t.Error("Command '%v' should have passed parsing", cmd)
		}
		if !sameWords(req.Arguments(), res) {
			t.Errorf("Arguments parsed from '%v' are not '%v'", cmd, res)
		}
	}

	test([]string{"stdinenabled", "value1", "value2"}, nil, []string{"value1", "value2"})

	fstdin := fileToSimulateStdin(t, "stdin1")

	test([]string{"stdinenabled"}, fstdin, []string{"stdin1"})
	test([]string{"stdinenabled", "value1"}, fstdin, []string{"stdin1", "value1"})
	test([]string{"stdinenabled", "value1", "value2"}, fstdin, []string{"stdin1", "value1", "value2"})
}
