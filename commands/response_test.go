package commands

import (
	"fmt"
	"testing"
)

type TestOutput struct {
	Foo, Bar string
	Baz      int
}

func TestMarshalling(t *testing.T) {
	req := NewEmptyRequest()

	res := NewResponse(req)
	res.SetValue(TestOutput{"beep", "boop", 1337})

	// get command global options so we can set the encoding option
	cmd := Command{}
	options, err := cmd.GetOptions(nil)
	if err != nil {
		t.Error(err)
	}

	_, err = res.Marshal()
	if err == nil {
		t.Error("Should have failed (no encoding type specified in request)")
	}

	req.SetOption(EncShort, JSON)
	req.ConvertOptions(options)

	bytes, err := res.Marshal()
	if err != nil {
		t.Error(err, "Should have passed")
	}
	output := string(bytes)
	if output != "{\"Foo\":\"beep\",\"Bar\":\"boop\",\"Baz\":1337}" {
		t.Error("Incorrect JSON output")
	}

	res.SetError(fmt.Errorf("You broke something!"), ErrClient)
	bytes, err = res.Marshal()
	if err != nil {
		t.Error("Should have passed")
	}
	output = string(bytes)
	if output != "{\"Message\":\"You broke something!\",\"Code\":1}" {
		t.Error("Incorrect JSON output")
	}
}
