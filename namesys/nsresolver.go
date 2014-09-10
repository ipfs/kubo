package namesys

type NSResolver interface {
	Resolve(string) (string, error)
}
