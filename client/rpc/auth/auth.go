package auth

import "net/http"

var _ http.RoundTripper = &AuthorizedRoundTripper{}

type AuthorizedRoundTripper struct {
	authorization string
	roundTripper  http.RoundTripper
}

// NewAuthorizedRoundTripper creates a new [http.RoundTripper] that will set the
// Authorization HTTP header with the value of [authorization]. The given [roundTripper] is
// the base [http.RoundTripper]. If it is nil, [http.DefaultTransport] is used.
func NewAuthorizedRoundTripper(authorization string, roundTripper http.RoundTripper) http.RoundTripper {
	if roundTripper == nil {
		roundTripper = http.DefaultTransport
	}

	return &AuthorizedRoundTripper{
		authorization: authorization,
		roundTripper:  roundTripper,
	}
}

func (tp *AuthorizedRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Header.Set("Authorization", tp.authorization)
	return tp.roundTripper.RoundTrip(r)
}
