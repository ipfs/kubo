package namesys

type Resolver interface {
	Resolve(string) (string, error)
	Matches(string) bool
}
