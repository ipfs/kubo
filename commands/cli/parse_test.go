package cli

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

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
		var opts map[string]interface{}
		var input []string

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

	test("test -", kvs{}, words{"-"})
	testFail("-b -b")
	test("test beep boop", kvs{}, words{"beep", "boop"})
	testFail("-s")
	test("-s foo", kvs{"s": "foo"}, words{})
	test("-sfoo", kvs{"s": "foo"}, words{})
	test("-s=foo", kvs{"s": "foo"}, words{})
	test("-b", kvs{"b": true}, words{})
	test("-bs foo", kvs{"b": true, "s": "foo"}, words{})
	test("-sb", kvs{"s": "b"}, words{})
	test("-b test foo", kvs{"b": true}, words{"foo"})
	test("--bool test foo", kvs{"bool": true}, words{"foo"})
	testFail("--bool=foo")
	testFail("--string")
	test("--string foo", kvs{"string": "foo"}, words{})
	test("--string=foo", kvs{"string": "foo"}, words{})
	test("-- -b", kvs{}, words{"-b"})
	test("test foo -b", kvs{"b": true}, words{"foo"})
	test("-b=false", kvs{"b": false}, words{})
	test("-b=true", kvs{"b": true}, words{})
	test("-b=false test foo", kvs{"b": false}, words{"foo"})
	test("-b=true test foo", kvs{"b": true}, words{"foo"})
	test("--bool=true test foo", kvs{"bool": true}, words{"foo"})
	test("--bool=false test foo", kvs{"bool": false}, words{"foo"})
	test("-b test true", kvs{"b": true}, words{"true"})
	test("-b test false", kvs{"b": true}, words{"false"})
	test("-b=FaLsE test foo", kvs{"b": false}, words{"foo"})
	test("-b=TrUe test foo", kvs{"b": true}, words{"foo"})
	test("-b test true", kvs{"b": true}, words{"true"})
	test("-b test false", kvs{"b": true}, words{"false"})
	test("-b --string foo test bar", kvs{"b": true, "string": "foo"}, words{"bar"})
	test("-b=false --string bar", kvs{"b": false, "string": "bar"}, words{})
	testFail("foo test")
}

func TestArgumentParsing(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stdin handling doesnt yet work on windows")
	}
	rootCmd := &commands.Command{
		Subcommands: map[string]*commands.Command{
			"noarg": {},
			"onearg": {
				Arguments: []commands.Argument{
					commands.StringArg("a", true, false, "some arg"),
				},
			},
			"twoargs": {
				Arguments: []commands.Argument{
					commands.StringArg("a", true, false, "some arg"),
					commands.StringArg("b", true, false, "another arg"),
				},
			},
			"variadic": {
				Arguments: []commands.Argument{
					commands.StringArg("a", true, true, "some arg"),
				},
			},
			"optional": {
				Arguments: []commands.Argument{
					commands.StringArg("b", false, true, "another arg"),
				},
			},
			"optionalsecond": {
				Arguments: []commands.Argument{
					commands.StringArg("a", true, false, "some arg"),
					commands.StringArg("b", false, false, "another arg"),
				},
			},
			"reversedoptional": {
				Arguments: []commands.Argument{
					commands.StringArg("a", false, false, "some arg"),
					commands.StringArg("b", true, false, "another arg"),
				},
			},
			"FileArg": {
				Arguments: []commands.Argument{
					commands.FileArg("a", true, false, "some arg"),
				},
			},
			"FileArg+Variadic": {
				Arguments: []commands.Argument{
					commands.FileArg("a", true, true, "some arg"),
				},
			},
			"FileArg+Stdin": {
				Arguments: []commands.Argument{
					commands.FileArg("a", true, true, "some arg").EnableStdin(),
				},
			},
			"StringArg+FileArg": {
				Arguments: []commands.Argument{
					commands.StringArg("a", true, false, "some arg"),
					commands.FileArg("a", true, false, "some arg"),
				},
			},
			"StringArg+FileArg+Stdin": {
				Arguments: []commands.Argument{
					commands.StringArg("a", true, false, "some arg"),
					commands.FileArg("a", true, true, "some arg").EnableStdin(),
				},
			},
			"StringArg+FileArg+Variadic": {
				Arguments: []commands.Argument{
					commands.StringArg("a", true, false, "some arg"),
					commands.FileArg("a", true, true, "some arg"),
				},
			},
			"StringArg+FileArg+Variadic+Stdin": {
				Arguments: []commands.Argument{
					commands.StringArg("a", true, false, "some arg"),
					commands.FileArg("a", true, true, "some arg"),
				},
			},
		},
	}

	test := func(cmd words, f *os.File, exp words) {
		if f != nil {
			if _, err := f.Seek(0, os.SEEK_SET); err != nil {
				t.Fatal(err)
			}
		}
		req, _, _, err := Parse(cmd, f, rootCmd)
		if err != nil {
			t.Errorf("Command '%v' should have passed parsing: %v", cmd, err)
		}

		parsedWords := make([]string, len(req.Arguments()))
		copy(parsedWords, req.Arguments())

		if files := req.Files(); files != nil {
			for file, err := files.NextFile(); err != io.EOF; file, err = files.NextFile() {
				parsedWords = append(parsedWords, file.FullPath())
			}
		}

		if !sameWords(parsedWords, exp) {
			t.Errorf("Arguments parsed from '%v' are '%v' instead of '%v'", cmd, parsedWords, exp)
		}
	}

	testFail := func(cmd words, fi *os.File, msg string) {
		_, _, _, err := Parse(cmd, nil, rootCmd)
		if err == nil {
			t.Errorf("Should have failed: %v", msg)
		}
	}

	test([]string{"noarg"}, nil, []string{})
	testFail([]string{"noarg", "value!"}, nil, "provided an arg, but command didn't define any")

	test([]string{"onearg", "value!"}, nil, []string{"value!"})
	testFail([]string{"onearg"}, nil, "didn't provide any args, arg is required")

	test([]string{"twoargs", "value1", "value2"}, nil, []string{"value1", "value2"})
	testFail([]string{"twoargs", "value!"}, nil, "only provided 1 arg, needs 2")
	testFail([]string{"twoargs"}, nil, "didn't provide any args, 2 required")

	test([]string{"variadic", "value!"}, nil, []string{"value!"})
	test([]string{"variadic", "value1", "value2", "value3"}, nil, []string{"value1", "value2", "value3"})
	testFail([]string{"variadic"}, nil, "didn't provide any args, 1 required")

	test([]string{"optional", "value!"}, nil, []string{"value!"})
	test([]string{"optional"}, nil, []string{})
	test([]string{"optional", "value1", "value2"}, nil, []string{"value1", "value2"})

	test([]string{"optionalsecond", "value!"}, nil, []string{"value!"})
	test([]string{"optionalsecond", "value1", "value2"}, nil, []string{"value1", "value2"})
	testFail([]string{"optionalsecond"}, nil, "didn't provide any args, 1 required")
	testFail([]string{"optionalsecond", "value1", "value2", "value3"}, nil, "provided too many args, takes 2 maximum")

	test([]string{"reversedoptional", "value1", "value2"}, nil, []string{"value1", "value2"})
	test([]string{"reversedoptional", "value!"}, nil, []string{"value!"})

	testFail([]string{"reversedoptional"}, nil, "didn't provide any args, 1 required")
	testFail([]string{"reversedoptional", "value1", "value2", "value3"}, nil, "provided too many args, only takes 1")

	// Since FileArgs are presently stored ordered by Path, the enum string
	// is used to construct a predictably ordered sequence of filenames.
	tmpFile := func(t *testing.T, enum string) *os.File {
		f, err := ioutil.TempFile("", enum)
		if err != nil {
			t.Fatal(err)
		}
		fn, err := filepath.EvalSymlinks(f.Name())
		if err != nil {
			t.Fatal(err)
		}
		f.Close()
		f, err = os.Create(fn)
		if err != nil {
			t.Fatal(err)
		}

		return f
	}
	file1 := tmpFile(t, "1")
	file2 := tmpFile(t, "2")
	file3 := tmpFile(t, "3")
	defer os.Remove(file3.Name())
	defer os.Remove(file2.Name())
	defer os.Remove(file1.Name())

	test([]string{"noarg"}, file1, []string{})
	test([]string{"FileArg", file1.Name()}, nil, []string{file1.Name()})
	test([]string{"FileArg+Variadic", file1.Name(), file2.Name()}, nil,
		[]string{file1.Name(), file2.Name()})
	test([]string{"FileArg+Stdin"}, file1, []string{file1.Name()})
	test([]string{"FileArg+Stdin", "-"}, file1, []string{file1.Name()})
	test([]string{"FileArg+Stdin", file1.Name(), "-"}, file2,
		[]string{file1.Name(), file2.Name()})
	test([]string{"StringArg+FileArg",
		"foo", file1.Name()}, nil, []string{"foo", file1.Name()})
	test([]string{"StringArg+FileArg+Variadic",
		"foo", file1.Name(), file2.Name()}, nil,
		[]string{"foo", file1.Name(), file2.Name()})
	test([]string{"StringArg+FileArg+Stdin",
		"foo", file1.Name(), "-"}, file2,
		[]string{"foo", file1.Name(), file2.Name()})
	test([]string{"StringArg+FileArg+Variadic+Stdin",
		"foo", file1.Name(), file2.Name()}, file3,
		[]string{"foo", file1.Name(), file2.Name()})
	test([]string{"StringArg+FileArg+Variadic+Stdin",
		"foo", file1.Name(), file2.Name(), "-"}, file3,
		[]string{"foo", file1.Name(), file2.Name(), file3.Name()})
}
