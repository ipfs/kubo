package rpc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/blang/semver/v4"
	"github.com/ipfs/boxo/files"
)

type RequestBuilder interface {
	Arguments(args ...string) RequestBuilder
	BodyString(body string) RequestBuilder
	BodyBytes(body []byte) RequestBuilder
	Body(body io.Reader) RequestBuilder
	FileBody(body io.Reader) RequestBuilder
	Option(key string, value interface{}) RequestBuilder
	Header(name, value string) RequestBuilder
	Send(ctx context.Context) (*Response, error)
	Exec(ctx context.Context, res interface{}) error
}

// encodedAbsolutePathVersion is the version from which the absolute path header in
// multipart requests is %-encoded. Before this version, its sent raw.
var encodedAbsolutePathVersion = semver.MustParse("0.23.0-dev")

// requestBuilder is an IPFS commands request builder.
type requestBuilder struct {
	command    string
	args       []string
	opts       map[string]string
	headers    map[string]string
	body       io.Reader
	buildError error

	shell *HttpApi
}

// Arguments adds the arguments to the args.
func (r *requestBuilder) Arguments(args ...string) RequestBuilder {
	r.args = append(r.args, args...)
	return r
}

// BodyString sets the request body to the given string.
func (r *requestBuilder) BodyString(body string) RequestBuilder {
	return r.Body(strings.NewReader(body))
}

// BodyBytes sets the request body to the given buffer.
func (r *requestBuilder) BodyBytes(body []byte) RequestBuilder {
	return r.Body(bytes.NewReader(body))
}

// Body sets the request body to the given reader.
func (r *requestBuilder) Body(body io.Reader) RequestBuilder {
	r.body = body
	return r
}

// FileBody sets the request body to the given reader wrapped into multipartreader.
func (r *requestBuilder) FileBody(body io.Reader) RequestBuilder {
	pr, _ := files.NewReaderPathFile("/dev/stdin", io.NopCloser(body), nil)
	d := files.NewMapDirectory(map[string]files.Node{"": pr})

	version, err := r.shell.loadRemoteVersion()
	if err != nil {
		// Unfortunately, we cannot return an error here. Changing this API is also
		// not the best since we would otherwise have an inconsistent RequestBuilder.
		// We save the error and return it when calling [requestBuilder.Send].
		r.buildError = err
		return r
	}

	useEncodedAbsPaths := version.LT(encodedAbsolutePathVersion)
	r.body = files.NewMultiFileReader(d, false, useEncodedAbsPaths)

	return r
}

// Option sets the given option.
func (r *requestBuilder) Option(key string, value interface{}) RequestBuilder {
	var s string
	switch v := value.(type) {
	case bool:
		s = strconv.FormatBool(v)
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		// slow case.
		s = fmt.Sprint(value)
	}
	if r.opts == nil {
		r.opts = make(map[string]string, 1)
	}
	r.opts[key] = s
	return r
}

// Header sets the given header.
func (r *requestBuilder) Header(name, value string) RequestBuilder {
	if r.headers == nil {
		r.headers = make(map[string]string, 1)
	}
	r.headers[name] = value
	return r
}

// Send sends the request and return the response.
func (r *requestBuilder) Send(ctx context.Context) (*Response, error) {
	if r.buildError != nil {
		return nil, r.buildError
	}

	r.shell.applyGlobal(r)

	req := NewRequest(ctx, r.shell.url, r.command, r.args...)
	req.Opts = r.opts
	req.Headers = r.headers
	req.Body = r.body
	return req.Send(&r.shell.httpcli)
}

// Exec sends the request a request and decodes the response.
func (r *requestBuilder) Exec(ctx context.Context, res interface{}) error {
	httpRes, err := r.Send(ctx)
	if err != nil {
		return err
	}

	if res == nil {
		lateErr := httpRes.Close()
		if httpRes.Error != nil {
			return httpRes.Error
		}
		return lateErr
	}

	return httpRes.decode(res)
}

var _ RequestBuilder = &requestBuilder{}
