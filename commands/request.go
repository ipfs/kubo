package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/repo/config"
	u "gx/ipfs/QmNiJuT8Ja3hMVpBHXv3Q6dwmperaQ6JjLtpMQgMCD7xvx/go-ipfs-util"

	"gx/ipfs/QmabLouZTZwhfALuBcssPvkzhbYGMb4394huT7HY4LQ6d3/go-ipfs-cmds"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit/files"
)

type Context struct {
	Online     bool
	ConfigRoot string
	ReqLog     *ReqLog

	config     *config.Config
	LoadConfig func(path string) (*config.Config, error)

	node          *core.IpfsNode
	ConstructNode func() (*core.IpfsNode, error)
}

// GetConfig returns the config of the current Command exection
// context. It may load it with the providied function.
func (c *Context) GetConfig() (*config.Config, error) {
	var err error
	if c.config == nil {
		if c.LoadConfig == nil {
			return nil, errors.New("nil LoadConfig function")
		}
		c.config, err = c.LoadConfig(c.ConfigRoot)
	}
	return c.config, err
}

// GetNode returns the node of the current Command exection
// context. It may construct it with the provided function.
func (c *Context) GetNode() (*core.IpfsNode, error) {
	var err error
	if c.node == nil {
		if c.ConstructNode == nil {
			return nil, errors.New("nil ConstructNode function")
		}
		c.node, err = c.ConstructNode()
	}
	return c.node, err
}

// NodeWithoutConstructing returns the underlying node variable
// so that clients may close it.
func (c *Context) NodeWithoutConstructing() *core.IpfsNode {
	return c.node
}

// Context returns the node's context.
func (c *Context) Context() context.Context {
	n, err := c.GetNode()
	if err != nil {
		log.Debug("error getting node: ", err)
		return context.Background()
	}

	return n.Context()
}

// LogRequest adds the passed request to the request log and
// returns a function that should be called when the request
// lifetime is over.
func (c *Context) LogRequest(req *cmds.Request) func() {
	rle := &ReqLogEntry{
		StartTime: time.Now(),
		Active:    true,
		Command:   strings.Join(req.Path, "/"),
		Options:   req.Options,
		Args:      req.Arguments,
		ID:        c.ReqLog.nextID,
		log:       c.ReqLog,
	}
	c.ReqLog.AddEntry(rle)

	return func() {
		c.ReqLog.Finish(rle)
	}
}

// Close cleans up the application state.
func (c *Context) Close() {
	// let's not forget teardown. If a node was initialized, we must close it.
	// Note that this means the underlying req.Context().Node variable is exposed.
	// this is gross, and should be changed when we extract out the exec Context.
	if c.node != nil {
		log.Info("Shutting down node...")
		c.node.Close()
	}
}

// Request represents a call to a command from a consumer
type Request interface {
	Path() []string
	Option(name string) *cmdkit.OptionValue
	Options() cmdkit.OptMap
	SetOption(name string, val interface{})
	SetOptions(opts cmdkit.OptMap) error
	Arguments() []string
	StringArguments() []string
	SetArguments([]string)
	Files() files.File
	SetFiles(files.File)
	Context() context.Context
	InvocContext() *Context
	SetInvocContext(Context)
	Command() *Command
	Values() map[string]interface{}
	Stdin() io.Reader
	VarArgs(func(string) error) error

	ConvertOptions() error
}

type request struct {
	path       []string
	options    cmdkit.OptMap
	arguments  []string
	files      files.File
	cmd        *Command
	ctx        Context
	rctx       context.Context
	optionDefs map[string]cmdkit.Option
	values     map[string]interface{}
	stdin      io.Reader
}

// Path returns the command path of this request
func (r *request) Path() []string {
	return r.path
}

// Option returns the value of the option for given name.
func (r *request) Option(name string) *cmdkit.OptionValue {
	// find the option with the specified name
	option, found := r.optionDefs[name]
	if !found {
		return nil
	}

	// try all the possible names, break if we find a value
	for _, n := range option.Names() {
		val, found := r.options[n]
		if found {
			return &cmdkit.OptionValue{
				Value:      val,
				ValueFound: found,
				Def:        option,
			}
		}
	}

	return &cmdkit.OptionValue{
		Value:      option.Default(),
		ValueFound: false,
		Def:        option,
	}
}

// Options returns a copy of the option map
func (r *request) Options() cmdkit.OptMap {
	output := make(cmdkit.OptMap)
	for k, v := range r.options {
		output[k] = v
	}
	return output
}

// SetOption sets the value of the option for given name.
func (r *request) SetOption(name string, val interface{}) {
	// find the option with the specified name
	option, found := r.optionDefs[name]
	if !found {
		return
	}

	// try all the possible names, if we already have a value then set over it
	for _, n := range option.Names() {
		_, found := r.options[n]
		if found {
			r.options[n] = val
			return
		}
	}

	r.options[name] = val
}

// SetOptions sets the option values, unsetting any values that were previously set
func (r *request) SetOptions(opts cmdkit.OptMap) error {
	r.options = opts
	return r.ConvertOptions()
}

func (r *request) StringArguments() []string {
	return r.arguments
}

// Arguments returns the arguments slice
func (r *request) Arguments() []string {
	if r.haveVarArgsFromStdin() {
		err := r.VarArgs(func(s string) error {
			r.arguments = append(r.arguments, s)
			return nil
		})
		if err != nil && err != io.EOF {
			log.Error(err)
		}
	}

	return r.arguments
}

func (r *request) SetArguments(args []string) {
	r.arguments = args
}

func (r *request) Files() files.File {
	return r.files
}

func (r *request) SetFiles(f files.File) {
	r.files = f
}

func (r *request) Context() context.Context {
	return r.rctx
}

func (r *request) haveVarArgsFromStdin() bool {
	// we expect varargs if we have a string argument that supports stdin
	// and not arguments to satisfy it
	if len(r.cmd.Arguments) == 0 {
		return false
	}

	last := r.cmd.Arguments[len(r.cmd.Arguments)-1]
	return last.SupportsStdin && last.Type == cmdkit.ArgString && (last.Required || last.Variadic) &&
		len(r.arguments) < len(r.cmd.Arguments)
}

// VarArgs can be used when you want string arguments as input
// and also want to be able to handle them in a streaming fashion
func (r *request) VarArgs(f func(string) error) error {
	if len(r.arguments) >= len(r.cmd.Arguments) {
		for _, arg := range r.arguments[len(r.cmd.Arguments)-1:] {
			err := f(arg)
			if err != nil {
				return err
			}
		}

		return nil
	}

	if r.files == nil {
		log.Warning("expected more arguments from stdin")
		return nil
	}

	fi, err := r.files.NextFile()
	if err != nil {
		return err
	}

	var any bool
	scan := bufio.NewScanner(fi)
	for scan.Scan() {
		any = true
		err := f(scan.Text())
		if err != nil {
			return err
		}
	}
	if !any {
		return f("")
	}

	return nil
}

func (r *request) InvocContext() *Context {
	return &r.ctx
}

func (r *request) SetInvocContext(ctx Context) {
	r.ctx = ctx
}

func (r *request) Command() *Command {
	return r.cmd
}

type converter func(string) (interface{}, error)

var converters = map[reflect.Kind]converter{
	cmdkit.Bool: func(v string) (interface{}, error) {
		if v == "" {
			return true, nil
		}
		return strconv.ParseBool(v)
	},
	cmdkit.Int: func(v string) (interface{}, error) {
		val, err := strconv.ParseInt(v, 0, 32)
		if err != nil {
			return nil, err
		}
		return int(val), err
	},
	cmdkit.Uint: func(v string) (interface{}, error) {
		val, err := strconv.ParseUint(v, 0, 32)
		if err != nil {
			return nil, err
		}
		return int(val), err
	},
	cmdkit.Float: func(v string) (interface{}, error) {
		return strconv.ParseFloat(v, 64)
	},
}

func (r *request) Values() map[string]interface{} {
	return r.values
}

func (r *request) Stdin() io.Reader {
	return r.stdin
}

func (r *request) ConvertOptions() error {
	for k, v := range r.options {
		opt, ok := r.optionDefs[k]
		if !ok {
			continue
		}

		kind := reflect.TypeOf(v).Kind()
		if kind != opt.Type() {
			if kind == cmdkit.String {
				convert := converters[opt.Type()]
				str, ok := v.(string)
				if !ok {
					return u.ErrCast()
				}
				val, err := convert(str)
				if err != nil {
					value := fmt.Sprintf("value '%v'", v)
					if len(str) == 0 {
						value = "empty value"
					}
					return fmt.Errorf("Could not convert %s to type '%s' (for option '-%s')",
						value, opt.Type().String(), k)
				}
				r.options[k] = val

			} else {
				return fmt.Errorf("Option '%s' should be type '%s', but got type '%s'",
					k, opt.Type().String(), kind.String())
			}
		} else {
			r.options[k] = v
		}

		for _, name := range opt.Names() {
			if _, ok := r.options[name]; name != k && ok {
				return fmt.Errorf("Duplicate command options were provided ('%s' and '%s')",
					k, name)
			}
		}
	}

	return nil
}

// NewEmptyRequest initializes an empty request
func NewEmptyRequest() (Request, error) {
	return NewRequest(nil, nil, nil, nil, nil, nil)
}

// NewRequest returns a request initialized with given arguments
// An non-nil error will be returned if the provided option values are invalid
func NewRequest(path []string, opts cmdkit.OptMap, args []string, file files.File, cmd *Command, optDefs map[string]cmdkit.Option) (Request, error) {
	if opts == nil {
		opts = make(cmdkit.OptMap)
	}
	if optDefs == nil {
		optDefs = make(map[string]cmdkit.Option)
	}

	ctx := Context{}
	values := make(map[string]interface{})
	req := &request{
		path:       path,
		options:    opts,
		arguments:  args,
		files:      file,
		cmd:        cmd,
		ctx:        ctx,
		optionDefs: optDefs,
		values:     values,
		stdin:      os.Stdin,
	}
	err := req.ConvertOptions()
	if err != nil {
		return nil, err
	}

	return req, nil
}
