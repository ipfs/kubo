package mfs

// Flags is an extensible struct used to pass open options.
type Flags struct {
	Read  bool
	Write bool
	Sync  bool
}
