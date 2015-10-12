package httpfile

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/anacrolix/missinggo"
)

type File struct {
	off    int64
	r      io.ReadCloser
	rOff   int64
	length int64
	url    string
}

func OpenSectionReader(url string, off, n int64) (ret io.ReadCloser, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+n-1))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	if resp.StatusCode == http.StatusNotFound {
		err = ErrNotFound
		resp.Body.Close()
		return
	}
	if resp.StatusCode != http.StatusPartialContent {
		err = fmt.Errorf("bad response status: %s", resp.Status)
		resp.Body.Close()
		return
	}
	ret = resp.Body
	return
}

func Open(url string) *File {
	return &File{
		url: url,
	}
}

func (me *File) prepareReader() (err error) {
	if me.r != nil && me.off != me.rOff {
		me.r.Close()
		me.r = nil
	}
	if me.r != nil {
		return nil
	}
	req, err := http.NewRequest("GET", me.url, nil)
	if err != nil {
		return
	}
	if me.off != 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", me.off))
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	switch resp.StatusCode {
	case http.StatusPartialContent:
		cr, ok := missinggo.ParseHTTPBytesContentRange(resp.Header.Get("Content-Range"))
		if !ok || cr.First != me.off {
			err = errors.New("bad response")
			resp.Body.Close()
			return
		}
		me.length = cr.Length
	case http.StatusOK:
		if me.off != 0 {
			err = errors.New("bad response")
			resp.Body.Close()
			return
		}
		if h := resp.Header.Get("Content-Length"); h != "" {
			var cl uint64
			cl, err = strconv.ParseUint(h, 10, 64)
			if err != nil {
				resp.Body.Close()
				return
			}
			me.length = int64(cl)
		}
	default:
		err = errors.New(resp.Status)
		resp.Body.Close()
		return
	}
	me.r = resp.Body
	me.rOff = me.off
	return
}

func (me *File) Read(b []byte) (n int, err error) {
	err = me.prepareReader()
	if err != nil {
		return
	}
	n, err = me.r.Read(b)
	me.off += int64(n)
	me.rOff += int64(n)
	return
}

func instanceLength(r *http.Response) (int64, error) {
	switch r.StatusCode {
	case http.StatusOK:
		if h := r.Header.Get("Content-Length"); h != "" {
			return strconv.ParseInt(h, 10, 64)
		} else {
			return -1, nil
		}
	case http.StatusPartialContent:
		cr, ok := missinggo.ParseHTTPBytesContentRange(r.Header.Get("Content-Range"))
		if !ok {
			return -1, errors.New("bad 206 response")
		}
		return cr.Length, nil
	default:
		return -1, errors.New(r.Status)
	}
}

func (me *File) Seek(offset int64, whence int) (ret int64, err error) {
	switch whence {
	case os.SEEK_SET:
		ret = offset
	case os.SEEK_CUR:
		ret = me.off + offset
	case os.SEEK_END:
		if me.length < 0 {
			err = errors.New("length unknown")
			return
		}
		ret = me.length + offset
	default:
		err = fmt.Errorf("unhandled whence: %d", whence)
		return
	}
	me.off = ret
	return
}

func (me *File) Write(b []byte) (n int, err error) {
	req, err := http.NewRequest("PATCH", me.url, bytes.NewReader(b))
	if err != nil {
		return
	}
	req.Header.Set("Content-Range", fmt.Sprintf("bytes=%d-", me.off))
	req.ContentLength = int64(len(b))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		err = errors.New(resp.Status)
		return
	}
	n = len(b)
	me.off += int64(n)
	return
}

var (
	ErrNotFound = errors.New("not found")
)

// Returns the length of the resource in bytes.
func GetLength(url string) (ret int64, err error) {
	resp, err := http.Head(url)
	if err != nil {
		return
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		err = ErrNotFound
		return
	}
	return instanceLength(resp)
}

func (me *File) Close() error {
	me.url = ""
	if me.r != nil {
		me.r.Close()
		me.r = nil
	}
	return nil
}

func Delete(urlStr string) (err error) {
	req, err := http.NewRequest("DELETE", urlStr, nil)
	if err != nil {
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		err = ErrNotFound
		return
	}
	if resp.StatusCode != 200 {
		err = fmt.Errorf("response: %s", resp.Status)
	}
	return
}
