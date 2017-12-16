package legacy

import (
	"bytes"
	"context"
	"io"
	"testing"

	oldcmds "github.com/ipfs/go-ipfs/commands"
	cmdkit "gx/ipfs/QmNaA1HxkbVtweGfabDMy2DMLvqQ1eg3LNEqDMVA3zCoz1/go-ipfs-cmdkit"
	cmds "gx/ipfs/QmXPgUkyFLMN3c79WrGM2VbjWynSPnmaHjF2AviBVQE2i7/go-ipfs-cmds"
)

type WriteNopCloser struct {
	io.Writer
}

func (wc WriteNopCloser) Close() error {
	return nil
}

func TestNewCommand(t *testing.T) {
	root := &cmds.Command{
		Subcommands: map[string]*cmds.Command{
			"test": NewCommand(&oldcmds.Command{
				Run: func(req oldcmds.Request, res oldcmds.Response) {
					res.SetOutput("Test.")
				},
				Marshalers: map[oldcmds.EncodingType]oldcmds.Marshaler{
					oldcmds.Text: func(res oldcmds.Response) (io.Reader, error) {
						ch, ok := res.Output().(<-chan interface{})
						if !ok {
							t.Fatalf("output is not <-chan interface{} but %T", ch)
						}

						v := <-ch
						str, ok := v.(string)
						if !ok {
							t.Fatalf("read value is not string but %T", v)
						}

						buf := bytes.NewBuffer(nil)
						_, err := io.WriteString(buf, str)
						if err != nil {
							t.Fatal(err)
						}

						return buf, nil
					},
				},
				Subcommands: map[string]*oldcmds.Command{
					"sub": &oldcmds.Command{
						Options: []cmdkit.Option{
							cmdkit.NewOption(cmdkit.String, "test", "t", "some random test flag"),
						},
					},
				},
			}),
		},
	}

	path := []string{"test"}
	req, err := cmds.NewRequest(context.TODO(), path, nil, nil, nil, root)
	if err != nil {
		t.Fatal(err)
	}

	buf := bytes.NewBuffer(nil)

	// test calling "test" command
	testCmd := root.Subcommand("test")
	enc := testCmd.Encoders[oldcmds.Text]
	if enc == nil {
		t.Fatal("got nil encoder")
	}

	re := cmds.NewWriterResponseEmitter(WriteNopCloser{buf}, req, enc)

	var env oldcmds.Context

	err = root.Call(req, re, &env)
	if err != nil {
		t.Fatal(err)
	}

	expected := `{"Value":"Test."}
`

	if buf.String() != expected {
		t.Fatalf("expected string %#v but got %#v", expected, buf.String())
	}

	// test getting subcommand
	subCmd := testCmd.Subcommand("sub")
	if subCmd == nil {
		t.Fatal("got nil subcommand")
	}

	if nOpts := len(subCmd.Options); nOpts != 1 {
		t.Fatalf("subcommand has %v options, expected 1", nOpts)
	}

	opt := subCmd.Options[0]

	if nNames := len(opt.Names()); nNames != 2 {
		t.Fatalf("option has %v names, expected 2", nNames)
	}

	names := opt.Names()
	if names[0] != "test" {
		t.Fatalf("option has name %q, expected %q", names[0], "test")
	}

	if names[1] != "t" {
		t.Fatalf("option has name %q, expected %q", names[1], "t")
	}
}

func TestPipePair(t *testing.T) {
	cmd := &cmds.Command{Type: "string"}

	req, err := cmds.NewRequest(context.TODO(), nil, nil, nil, nil, cmd)
	if err != nil {
		t.Fatal(err)
	}

	r, w := io.Pipe()
	re := cmds.NewWriterResponseEmitter(w, req, cmds.Encoders[cmds.JSON])
	res := cmds.NewReaderResponse(r, cmds.JSON, req)

	wait := make(chan interface{})

	expect := "abc"
	go func() {
		err := re.Emit(expect)
		if err != nil {
			t.Fatal(err)
		}

		close(wait)
	}()

	v, err := res.Next()
	if err != nil {
		t.Fatal(err)
	}
	str, ok := v.(*string)
	if !ok {
		t.Fatalf("expected type %T but got %T", expect, v)
	}
	if *str != expect {
		t.Fatalf("expected value %#v but got %#v", expect, v)
	}

	<-wait

}

func TestTeeEmitter(t *testing.T) {
	req, err := cmds.NewRequest(nil, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	buf1 := bytes.NewBuffer(nil)
	re1 := cmds.NewWriterResponseEmitter(WriteNopCloser{buf1}, req, cmds.Encoders[cmds.Text])

	buf2 := bytes.NewBuffer(nil)
	re2 := cmds.NewWriterResponseEmitter(WriteNopCloser{buf2}, req, cmds.Encoders[cmds.Text])

	re := cmds.NewTeeEmitter(re1, re2)

	expect := "def"
	err = re.Emit(expect)
	if err != nil {
		t.Fatal(err)
	}

	if buf1.String() != expect {
		t.Fatal("expected %#v, got %#v", expect, buf1.String())
	}

	if buf2.String() != expect {
		t.Fatal("expected %#v, got %#v", expect, buf2.String())
	}
}

/*
type teeErrorTestCase struct {
	err1, err2 error
	bothNil    bool
	errString  string
}

func TestTeeError(t *testing.T) {
	tcs := []teeErrorTestCase{
		teeErrorTestCase{nil, nil, true, ""},
		teeErrorTestCase{fmt.Errorf("error!"), nil, false, "1: error!"},
		teeErrorTestCase{nil, fmt.Errorf("error!"), false, "2: error!"},
		teeErrorTestCase{fmt.Errorf("error!"), fmt.Errorf("error!"), false, `1: error!
2: error!`},
	}

	for i, tc := range tcs {
		teeError := cmds.TeeError{tc.err1, tc.err2}
		if teeError.BothNil() != tc.bothNil {
			t.Fatalf("BothNil()/%d: expected %v but got %v", i, tc.bothNil, teeError.BothNil())
		}

		if teeError.Error() != tc.errString {
			t.Fatalf("Error()/%d: expected %v but got %v", i, tc.errString, teeError.Error())
		}
	}
}
*/
