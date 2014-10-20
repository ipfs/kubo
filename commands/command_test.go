package commands

import "testing"

func TestOptionValidation(t *testing.T) {
	cmd := Command{
		Options: []Option{
			Option{[]string{"b", "beep"}, Int},
			Option{[]string{"B", "boop"}, String},
		},
		run: func(req Request, res Response) {},
	}

	req := NewEmptyRequest()
	req.SetOption("foo", 5)
	res := cmd.Call(req, nil)
	if res.Error() == nil {
		t.Error("Should have failed (unrecognized option)")
	}

	req = NewEmptyRequest()
	req.SetOption("beep", 5)
	req.SetOption("b", 10)
	res = cmd.Call(req, nil)
	if res.Error() == nil {
		t.Error("Should have failed (duplicate options)")
	}

	req = NewEmptyRequest()
	req.SetOption("beep", "foo")
	res = cmd.Call(req, nil)
	if res.Error() == nil {
		t.Error("Should have failed (incorrect type)")
	}

	req = NewEmptyRequest()
	req.SetOption("beep", 5)
	res = cmd.Call(req, nil)
	if res.Error() != nil {
		t.Error(res.Error(), "Should have passed")
	}

	req = NewEmptyRequest()
	req.SetOption("beep", 5)
	req.SetOption("boop", "test")
	res = cmd.Call(req, nil)
	if res.Error() != nil {
		t.Error("Should have passed")
	}

	req = NewEmptyRequest()
	req.SetOption("b", 5)
	req.SetOption("B", "test")
	res = cmd.Call(req, nil)
	if res.Error() != nil {
		t.Error("Should have passed")
	}

	req = NewEmptyRequest()
	req.SetOption(EncShort, "json")
	res = cmd.Call(req, nil)
	if res.Error() != nil {
		t.Error("Should have passed")
	}

	req = NewEmptyRequest()
	req.SetOption("b", "100")
	res = cmd.Call(req, nil)
	if res.Error() != nil {
		t.Error("Should have passed")
	}

	req = NewEmptyRequest()
	req.SetOption("b", ":)")
	res = cmd.Call(req, nil)
	if res.Error == nil {
		t.Error(res.Error, "Should have failed (string value not convertible to int)")
	}
}

func TestRegistration(t *testing.T) {
	noop := func(req Request, res Response) {}
	cmds := []*Command{
		&Command{
			Options: []Option{
				Option{[]string{"beep"}, Int},
			},
			run: noop,
		},

		&Command{
			Options: []Option{
				Option{[]string{"boop"}, Int},
			},
			run: noop,
		},

		&Command{
			Options: []Option{
				Option{[]string{"boop"}, String},
			},
			run: noop,
		},

		&Command{
			Options: []Option{
				Option{[]string{"bop"}, String},
			},
			run: noop,
		},

		&Command{
			Options: []Option{
				Option{[]string{EncShort}, String},
			},
			run: noop,
		},
	}

	err := cmds[0].Register("foo", cmds[1])
	if err != nil {
		t.Error("Should have passed")
	}

	err = cmds[0].Register("bar", cmds[2])
	if err == nil {
		t.Error("Should have failed (option name collision)")
	}

	err = cmds[0].Register("foo", cmds[3])
	if err == nil {
		t.Error("Should have failed (subcommand name collision)")
	}

	err = cmds[0].Register("baz", cmds[4])
	if err == nil {
		t.Error("Should have failed (option name collision with global options)")
	}
}

func TestResolving(t *testing.T) {
	cmd := &Command{}
	cmdA := &Command{}
	cmdB := &Command{}
	cmdB2 := &Command{}
	cmdC := &Command{}

	cmd.Register("a", cmdA)
	cmdA.Register("B", cmdB2)
	cmdA.Register("b", cmdB)
	cmdB.Register("c", cmdC)

	cmds, err := cmd.Resolve([]string{"a", "b", "c"})
	if err != nil {
		t.Error(err)
	}
	if len(cmds) != 4 || cmds[0] != cmd || cmds[1] != cmdA || cmds[2] != cmdB || cmds[3] != cmdC {
		t.Error("Returned command path is different than expected", cmds)
	}
}
