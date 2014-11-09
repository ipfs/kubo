package commands

import (
	"fmt"
	"strings"
	"testing"
)

type TestOutput struct {
	Foo, Bar string
	Baz      int
}

func TestMarshalling(t *testing.T) {
	req := NewEmptyRequest()

	res := NewResponse(req)
	res.SetOutput(TestOutput{"beep", "boop", 1337})

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
	if removeWhitespace(output) != "{\"Foo\":\"beep\",\"Bar\":\"boop\",\"Baz\":1337}" {
		t.Error("Incorrect JSON output")
	}

	res.SetError(fmt.Errorf("Oops!"), ErrClient)
	bytes, err = res.Marshal()
	if err != nil {
		t.Error("Should have passed")
	}
	output = string(bytes)
	fmt.Println(removeWhitespace(output))
	if removeWhitespace(output) != "{\"Message\":\"Oops!\",\"Code\":1}" {
		t.Error("Incorrect JSON output")
	}
}

func removeWhitespace(input string) string {
	input = strings.Replace(input, " ", "", -1)
	input = strings.Replace(input, "\t", "", -1)
	input = strings.Replace(input, "\n", "", -1)
	return strings.Replace(input, "\r", "", -1)
}
