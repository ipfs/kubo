package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"

	files "github.com/ipfs/go-ipfs-files"
)

type trailerReader struct {
	resp *http.Response
}

func (r *trailerReader) Read(b []byte) (int, error) {
	n, err := r.resp.Body.Read(b)
	if err != nil {
		if e := r.resp.Trailer.Get("X-Stream-Error"); e != "" {
			err = errors.New(e)
		}
	}
	return n, err
}

func (r *trailerReader) Close() error {
	return r.resp.Body.Close()
}

type Response struct {
	Output io.ReadCloser
	Error  *Error
}

func (r *Response) Close() error {
	if r.Output != nil {

		// always drain output (response body) //TODO: make optional for things like cat
		_, err1 := io.Copy(ioutil.Discard, r.Output)
		err2 := r.Output.Close()
		if err1 != nil {
			return err1
		}
		return err2
	}
	return nil
}

func (r *Response) Decode(dec interface{}) error {
	defer r.Close()
	if r.Error != nil {
		return r.Error
	}

	n := 0
	var err error
	for {
		err = json.NewDecoder(r.Output).Decode(dec)
		if err != nil {
			break
		}
		n++
	}
	if n > 0 && err == io.EOF {
		err = nil
	}
	return err
}

type Error struct {
	Command string
	Message string
	Code    int
}

func (e *Error) Error() string {
	var out string
	if e.Code != 0 {
		out = fmt.Sprintf("%s%d: ", out, e.Code)
	}
	return out + e.Message
}

func (r *Request) Send(c *http.Client) (*Response, error) {
	url := r.getURL()
	req, err := http.NewRequest("POST", url, r.Body)
	if err != nil {
		return nil, err
	}

	// Add any headers that were supplied via the RequestBuilder.
	for k, v := range r.Headers {
		req.Header.Add(k, v)
	}

	if fr, ok := r.Body.(*files.MultiFileReader); ok {
		req.Header.Set("Content-Type", "multipart/form-data; boundary="+fr.Boundary())
		req.Header.Set("Content-Disposition", "form-data: name=\"files\"")
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}

	contentType := resp.Header.Get("Content-Type")
	parts := strings.Split(contentType, ";")
	contentType = parts[0]

	nresp := new(Response)

	nresp.Output = &trailerReader{resp}
	if resp.StatusCode >= http.StatusBadRequest {
		e := &Error{
			Command: r.Command,
		}
		switch {
		case resp.StatusCode == http.StatusNotFound:
			e.Message = "command not found"
		case contentType == "text/plain":
			out, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ipfs-shell: warning! response (%d) read error: %s\n", resp.StatusCode, err)
			}
			e.Message = string(out)
		case contentType == "application/json":
			if err = json.NewDecoder(resp.Body).Decode(e); err != nil {
				fmt.Fprintf(os.Stderr, "ipfs-shell: warning! response (%d) unmarshall error: %s\n", resp.StatusCode, err)
			}
		default:
			fmt.Fprintf(os.Stderr, "ipfs-shell: warning! unhandled response (%d) encoding: %s", resp.StatusCode, contentType)
			out, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ipfs-shell: response (%d) read error: %s\n", resp.StatusCode, err)
			}
			e.Message = fmt.Sprintf("unknown ipfs-shell error encoding: %q - %q", contentType, out)
		}
		nresp.Error = e
		nresp.Output = nil

		// drain body and close
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
	}

	return nresp, nil
}

func (r *Request) getURL() string {

	values := make(url.Values)
	for _, arg := range r.Args {
		values.Add("arg", arg)
	}
	for k, v := range r.Opts {
		values.Add(k, v)
	}

	return fmt.Sprintf("%s/%s?%s", r.ApiBase, r.Command, values.Encode())
}
