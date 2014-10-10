package commands

import "testing"

func TestOptionValidation(t *testing.T) {
	cmd := Command{
		Options: []Option{
			Option{[]string{"b", "beep"}, Int},
			Option{[]string{"B", "boop"}, String},
		},
		f: func(req *Request, res *Response) {},
	}

	req := NewRequest()
	req.options["foo"] = 5
	res := cmd.Call(nil, req)
	if res.Error == nil {
		t.Error("Should have failed (unrecognized command)")
	}

	req = NewRequest()
	req.options["beep"] = 5
	req.options["b"] = 10
	res = cmd.Call(nil, req)
	if res.Error == nil {
		t.Error("Should have failed (duplicate options)")
	}

	req = NewRequest()
	req.options["beep"] = "foo"
	res = cmd.Call(nil, req)
	if res.Error == nil {
		t.Error("Should have failed (incorrect type)")
	}

	req = NewRequest()
	req.options["beep"] = 5
	res = cmd.Call(nil, req)
	if res.Error != nil {
		t.Error("Should have passed")
	}

	req = NewRequest()
	req.options["beep"] = 5
	req.options["boop"] = "test"
	res = cmd.Call(nil, req)
	if res.Error != nil {
		t.Error("Should have passed")
	}

	req = NewRequest()
	req.options["b"] = 5
	req.options["B"] = "test"
	res = cmd.Call(nil, req)
	if res.Error != nil {
		t.Error("Should have passed")
	}

	req = NewRequest()
	req.options["enc"] = "json"
	res = cmd.Call(nil, req)
	if res.Error != nil {
		t.Error("Should have passed")
	}
}

func TestRegistration(t *testing.T) {
	cmds := []*Command{
		&Command{
			Options: []Option{
				Option{[]string{"beep"}, Int},
			},
			f: func(req *Request, res *Response) {},
		},

		&Command{
			Options: []Option{
				Option{[]string{"boop"}, Int},
			},
			f: func(req *Request, res *Response) {},
		},

		&Command{
			Options: []Option{
				Option{[]string{"boop"}, String},
			},
			f: func(req *Request, res *Response) {},
		},

		&Command{
			Options: []Option{
				Option{[]string{"bop"}, String},
			},
			f: func(req *Request, res *Response) {},
		},

		&Command{
			Options: []Option{
				Option{[]string{"enc"}, String},
			},
			f: func(req *Request, res *Response) {},
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
