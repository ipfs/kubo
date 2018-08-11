package legacy

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"

	"gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit"
	"gx/ipfs/QmPVqQHEfLpqK7JLCsUkyam7rhuV3MAeZ9gueQQCrBwCta/go-ipfs-cmdkit/files"
	"gx/ipfs/QmUQb3xtNzkQCgTj2NjaqcJZNv2nfSSub2QAdy9DtQMRBT/go-ipfs-cmds"

	oldcmds "github.com/ipfs/go-ipfs/commands"
)

// requestWrapper implements a oldcmds.Request from an Request
type requestWrapper struct {
	req *cmds.Request
	ctx *oldcmds.Context
}

func (r *requestWrapper) String() string {
	return fmt.Sprintf("{%v, %v}", r.req, r.ctx)
}

func (r *requestWrapper) GoString() string {
	return fmt.Sprintf("lgc.Request{%#v, %#v}", r.req, r.ctx)
}

// InvocContext retuns the invocation context of the oldcmds.Request.
// It is faked using OldContext().
func (r *requestWrapper) InvocContext() *oldcmds.Context {
	return r.ctx
}

// SetInvocContext sets the invocation context. First the context is converted
// to a Context using NewContext().
func (r *requestWrapper) SetInvocContext(ctx oldcmds.Context) {
	r.ctx = &ctx
}

// Command is an empty stub.
func (r *requestWrapper) Command() *oldcmds.Command { return nil }

func (r *requestWrapper) Arguments() []string {
	cmdArgs := r.req.Command.Arguments
	reqArgs := r.req.Arguments

	// TODO figure out the exaclt policy for when to use these automatically
	// TODO once that's done, change the log.Debug below to log.Error
	// read arguments from body if we don't have all of them or the command has variadic arguemnts
	if len(reqArgs) < len(cmdArgs) ||
		len(cmdArgs) > 0 && cmdArgs[len(cmdArgs)-1].Variadic {
		err := r.req.ParseBodyArgs()
		if err != nil {
			log.Debug("error reading arguments from stdin: ", err)
		}
	}
	return r.req.Arguments
}

func (r *requestWrapper) Context() context.Context {
	return r.req.Context
}

func (r *requestWrapper) ConvertOptions() error {
	return convertOptions(r.req)
}

func (r *requestWrapper) Files() files.File {
	return r.req.Files
}

func (r *requestWrapper) Option(name string) *cmdkit.OptionValue {
	var option cmdkit.Option

	optDefs, err := r.req.Root.GetOptions(r.req.Path)
	if err != nil {
		return &cmdkit.OptionValue{}
	}
	for _, def := range optDefs {
		for _, optName := range def.Names() {
			if name == optName {
				option = def
				break
			}
		}
	}
	if option == nil {
		return nil
	}

	// try all the possible names, break if we find a value
	for _, n := range option.Names() {
		val, found := r.req.Options[n]
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

func (r *requestWrapper) Options() cmdkit.OptMap {
	return r.req.Options
}

func (r *requestWrapper) Path() []string {
	return r.req.Path
}

func (r *requestWrapper) SetArguments(args []string) {
	r.req.Arguments = args
}

func (r *requestWrapper) SetFiles(f files.File) {
	r.req.Files = f
}

func (r *requestWrapper) SetOption(name string, v interface{}) {
	r.req.SetOption(name, v)
}

func (r *requestWrapper) SetOptions(om cmdkit.OptMap) error {
	r.req.Options = om
	return convertOptions(r.req)
}

func (r *requestWrapper) Stdin() io.Reader {
	return os.Stdin
}

func (r *requestWrapper) StringArguments() []string {
	return r.req.Arguments
}

func (r *requestWrapper) Values() map[string]interface{} {
	return nil
}

// copied from go-ipfs-cmds/request.go
func convertOptions(req *cmds.Request) error {
	optDefSlice := req.Command.Options

	optDefs := make(map[string]cmdkit.Option)
	for _, def := range optDefSlice {
		for _, name := range def.Names() {
			optDefs[name] = def
		}
	}

	for k, v := range req.Options {
		opt, ok := optDefs[k]
		if !ok {
			continue
		}

		kind := reflect.TypeOf(v).Kind()
		if kind != opt.Type() {
			if str, ok := v.(string); ok {
				val, err := opt.Parse(str)
				if err != nil {
					value := fmt.Sprintf("value %q", v)
					if len(str) == 0 {
						value = "empty value"
					}
					return fmt.Errorf("could not convert %q to type %q (for option %q)",
						value, opt.Type().String(), "-"+k)
				}
				req.Options[k] = val

			} else {
				return fmt.Errorf("option %q should be type %q, but got type %q",
					k, opt.Type().String(), kind.String())
			}
		}

		for _, name := range opt.Names() {
			if _, ok := req.Options[name]; name != k && ok {
				return fmt.Errorf("duplicate command options were provided (%q and %q)",
					k, name)
			}
		}
	}

	return nil
}
