package namesys

type Resolver interface {
	// Resolve returns a base58 encoded string
	Resolve(string) (string, error)

	Matches(string) bool
}
