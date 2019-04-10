package httpapi

import "net/http"

type transport struct {
	header, value string
	httptr        http.RoundTripper
}

func newAuthenticatedTransport(tr http.RoundTripper, header, value string) *transport {
	return &transport{
		header: header,
		value:  value,
		httptr: tr,
	}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set(t.header, t.value)
	return t.httptr.RoundTrip(req)
}
