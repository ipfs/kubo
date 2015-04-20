package config

// Gateway contains options for the HTTP gateway server.
type Gateway struct {
	RootRedirect string
	Writable     bool
}
