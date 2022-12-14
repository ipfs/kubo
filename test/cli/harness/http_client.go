package harness

import (
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"
)

// HTTPClient is an HTTP client with some conveniences for testing.
// URLs are constructed from a base URL.
// The response body is buffered into a string.
// Internal errors cause panics so that tests don't need to check errors.
// The paths are evaluated as Go templates for readable string interpolation.
type HTTPClient struct {
	Client  *http.Client
	BaseURL string

	Timeout      time.Duration
	TemplateData any
}

type HTTPResponse struct {
	Body       string
	StatusCode int
	Headers    http.Header

	// The raw response. The body will be closed on this response.
	Resp *http.Response
}

func (c *HTTPClient) WithHeader(k, v string) func(h *http.Request) {
	return func(h *http.Request) {
		h.Header.Add(k, v)
	}
}

func (c *HTTPClient) DisableRedirects() *HTTPClient {
	c.Client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return c
}

// Do executes the request unchanged.
func (c *HTTPClient) Do(req *http.Request) *HTTPResponse {
	log.Debugf("making HTTP req %s to %q with headers %+v", req.Method, req.URL.String(), req.Header)
	resp, err := c.Client.Do(req)
	if resp != nil && resp.Body != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		panic(err)
	}
	bodyStr, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	return &HTTPResponse{
		Body:       string(bodyStr),
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Resp:       resp,
	}
}

// BuildURL constructs a request URL from the given path by interpolating the string and then appending it to the base URL.
func (c *HTTPClient) BuildURL(urlPath string) string {
	sb := &strings.Builder{}
	err := template.Must(template.New("test").Parse(urlPath)).Execute(sb, c.TemplateData)
	if err != nil {
		panic(err)
	}
	renderedPath := sb.String()
	return c.BaseURL + renderedPath
}

func (c *HTTPClient) Get(urlPath string, opts ...func(*http.Request)) *HTTPResponse {
	req, err := http.NewRequest(http.MethodGet, c.BuildURL(urlPath), nil)
	if err != nil {
		panic(err)
	}
	for _, o := range opts {
		o(req)
	}
	return c.Do(req)
}

func (c *HTTPClient) Post(urlPath string, body io.Reader, opts ...func(*http.Request)) *HTTPResponse {
	req, err := http.NewRequest(http.MethodPost, c.BuildURL(urlPath), body)
	if err != nil {
		panic(err)
	}
	for _, o := range opts {
		o(req)
	}
	return c.Do(req)
}

func (c *HTTPClient) PostStr(urlpath, body string, opts ...func(*http.Request)) *HTTPResponse {
	r := strings.NewReader(body)
	return c.Post(urlpath, r, opts...)
}

func (c *HTTPClient) Head(urlPath string, opts ...func(*http.Request)) *HTTPResponse {
	req, err := http.NewRequest(http.MethodHead, c.BuildURL(urlPath), nil)
	if err != nil {
		panic(err)
	}
	for _, o := range opts {
		o(req)
	}
	return c.Do(req)
}
