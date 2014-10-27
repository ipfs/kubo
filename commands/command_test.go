package commands

import "testing"

func TestOptionValidation(t *testing.T) {
	cmd := Command{
		Options: []Option{
			Option{[]string{"b", "beep"}, Int},
			Option{[]string{"B", "boop"}, String},
		},
		Run: func(req Request, res Response) {},
	}

	req := NewEmptyRequest()
	req.SetOption("beep", 5)
	req.SetOption("b", 10)
	res := cmd.Call(req)
	if res.Error() == nil {
		t.Error("Should have failed (duplicate options)")
	}

	req = NewEmptyRequest()
	req.SetOption("beep", "foo")
	res = cmd.Call(req)
	if res.Error() == nil {
		t.Error("Should have failed (incorrect type)")
	}

	req = NewEmptyRequest()
	req.SetOption("beep", 5)
	res = cmd.Call(req)
	if res.Error() != nil {
		t.Error(res.Error(), "Should have passed")
	}

	req = NewEmptyRequest()
	req.SetOption("beep", 5)
	req.SetOption("boop", "test")
	res = cmd.Call(req)
	if res.Error() != nil {
		t.Error("Should have passed")
	}

	req = NewEmptyRequest()
	req.SetOption("b", 5)
	req.SetOption("B", "test")
	res = cmd.Call(req)
	if res.Error() != nil {
		t.Error("Should have passed")
	}

	req = NewEmptyRequest()
	req.SetOption("foo", 5)
	res = cmd.Call(req)
	if res.Error() != nil {
		t.Error("Should have passed")
	}

	req = NewEmptyRequest()
	req.SetOption(EncShort, "json")
	res = cmd.Call(req)
	if res.Error() != nil {
		t.Error("Should have passed")
	}

	req = NewEmptyRequest()
	req.SetOption("b", "100")
	res = cmd.Call(req)
	if res.Error() != nil {
		t.Error("Should have passed")
	}

	req = NewEmptyRequest()
	req.SetOption("b", ":)")
	res = cmd.Call(req)
	if res.Error() == nil {
		t.Error(res.Error(), "Should have failed (string value not convertible to int)")
	}
}

func TestRegistration(t *testing.T) {
	noop := func(req Request, res Response) {}

	cmdA := &Command{
		Options: []Option{
			Option{[]string{"beep"}, Int},
		},
		Run: noop,
	}

	cmdB := &Command{
		Options: []Option{
			Option{[]string{"beep"}, Int},
		},
		Run: noop,
		Subcommands: map[string]*Command{
			"a": cmdA,
		},
	}

	cmdC := &Command{
		Options: []Option{
			Option{[]string{"encoding"}, String},
		},
		Run: noop,
	}

	res := cmdB.Call(NewRequest([]string{"a"}, nil, nil, nil))
	if res.Error() == nil {
		t.Error("Should have failed (option name collision)")
	}

	res = cmdC.Call(NewEmptyRequest())
	if res.Error() == nil {
		t.Error("Should have failed (option name collision with global options)")
	}
}

func TestResolving(t *testing.T) {
	cmdC := &Command{}
	cmdB := &Command{
		Subcommands: map[string]*Command{
			"c": cmdC,
		},
	}
	cmdB2 := &Command{}
	cmdA := &Command{
		Subcommands: map[string]*Command{
			"b": cmdB,
			"B": cmdB2,
		},
	}
	cmd := &Command{
		Subcommands: map[string]*Command{
			"a": cmdA,
		},
	}

	cmds, err := cmd.Resolve([]string{"a", "b", "c"})
	if err != nil {
		t.Error(err)
	}
	if len(cmds) != 4 || cmds[0] != cmd || cmds[1] != cmdA || cmds[2] != cmdB || cmds[3] != cmdC {
		t.Error("Returned command path is different than expected", cmds)
	}
}
