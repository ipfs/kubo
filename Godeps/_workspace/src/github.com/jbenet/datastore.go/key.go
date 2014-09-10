package datastore

import (
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/code.google.com/p/go-uuid/uuid"
	"path"
	"strings"
)

/*
A Key represents the unique identifier of an object.
Our Key scheme is inspired by file systems and Google App Engine key model.

Keys are meant to be unique across a system. Keys are hierarchical,
incorporating more and more specific namespaces. Thus keys can be deemed
'children' or 'ancestors' of other keys::

    Key("/Comedy")
    Key("/Comedy/MontyPython")

Also, every namespace can be parametrized to embed relevant object
information. For example, the Key `name` (most specific namespace) could
include the object type::

    Key("/Comedy/MontyPython/Actor:JohnCleese")
    Key("/Comedy/MontyPython/Sketch:CheeseShop")
    Key("/Comedy/MontyPython/Sketch:CheeseShop/Character:Mousebender")

*/
type Key struct {
	string
}

func NewKey(s string) Key {
	k := Key{s}
	k.Clean()
	return k
}

// Cleans up a Key, using path.Clean.
func (k *Key) Clean() {
	k.string = path.Clean("/" + k.string)
}

// Returns the string value of Key
func (k Key) String() string {
	return k.string
}

// Returns the bytes value of Key
func (k Key) Bytes() []byte {
	return []byte(k.string)
}

// Returns the `list` representation of this Key.
// NewKey("/Comedy/MontyPython/Actor:JohnCleese").List()
// ["Comedy", "MontyPythong", "Actor:JohnCleese"]
func (k Key) List() []string {
	return strings.Split(k.string, "/")[1:]
}

// Returns the reverse of this Key.
// NewKey("/Comedy/MontyPython/Actor:JohnCleese").Reverse()
// NewKey("/Actor:JohnCleese/MontyPython/Comedy")
func (k Key) Reverse() Key {
	l := k.List()
	r := make([]string, len(l), len(l))
	for i, e := range l {
		r[len(l)-i-1] = e
	}
	return NewKey(strings.Join(r, "/"))
}

// Returns the `namespaces` making up this Key.
// NewKey("/Comedy/MontyPython/Actor:JohnCleese").List()
// ["Comedy", "MontyPythong", "Actor:JohnCleese"]
func (k Key) Namespaces() []string {
	return k.List()
}

// Returns the "base" namespace of this key (like path.Base(filename))
// NewKey("/Comedy/MontyPython/Actor:JohnCleese").BaseNamespace()
// "Actor:JohnCleese"
func (k Key) BaseNamespace() string {
	n := k.Namespaces()
	return n[len(n)-1]
}

// Returns the "type" of this key (value of last namespace).
// NewKey("/Comedy/MontyPython/Actor:JohnCleese").List()
// "Actor"
func (k Key) Type() string {
	return NamespaceType(k.BaseNamespace())
}

// Returns the "name" of this key (field of last namespace).
// NewKey("/Comedy/MontyPython/Actor:JohnCleese").List()
// "Actor"
func (k Key) Name() string {
	return NamespaceValue(k.BaseNamespace())
}

// Returns an "instance" of this type key (appends value to namespace).
// NewKey("/Comedy/MontyPython/Actor:JohnCleese").List()
// "JohnCleese"
func (k Key) Instance(s string) Key {
	return NewKey(k.string + ":" + s)
}

// Returns the "path" of this key (parent + type).
// NewKey("/Comedy/MontyPython/Actor:JohnCleese").Path()
// NewKey("/Comedy/MontyPython/Actor")
func (k Key) Path() Key {
	s := k.Parent().string + "/" + NamespaceType(k.BaseNamespace())
	return NewKey(s)
}

// Returns the `parent` Key of this Key.
// NewKey("/Comedy/MontyPython/Actor:JohnCleese").Parent()
// NewKey("/Comedy/MontyPython")
func (k Key) Parent() Key {
	n := k.List()
	if len(n) == 1 {
		return NewKey("/")
	}
	return NewKey(strings.Join(n[:len(n)-1], "/"))
}

// Returns the `child` Key of this Key.
// NewKey("/Comedy/MontyPython").Child("Actor:JohnCleese")
// NewKey("/Comedy/MontyPython/Actor:JohnCleese")
func (k Key) Child(s string) Key {
	return NewKey(k.string + "/" + s)
}

// Returns whether this key is an ancestor of `other`
// NewKey("/Comedy").IsAncestorOf("/Comedy/MontyPython")
// true
func (k Key) IsAncestorOf(other Key) bool {
	if other.string == k.string {
		return false
	}
	return strings.HasPrefix(other.string, k.string)
}

// Returns whether this key is a descendent of `other`
// NewKey("/Comedy/MontyPython").IsDescendantOf("/Comedy")
// true
func (k Key) IsDescendantOf(other Key) bool {
	if other.string == k.string {
		return false
	}
	return strings.HasPrefix(k.string, other.string)
}

func (k Key) IsTopLevel() bool {
	return len(k.List()) == 1
}

// Returns a randomly (uuid) generated key.
// RandomKey()
// NewKey("/f98719ea086343f7b71f32ea9d9d521d")
func RandomKey() Key {
	return NewKey(strings.Replace(uuid.New(), "-", "", -1))
}

/*
A Key Namespace is like a path element.
A namespace can optionally include a type (delimited by ':')

    > NamespaceValue("Song:PhilosopherSong")
    PhilosopherSong
    > NamespaceType("Song:PhilosopherSong")
    Song
    > NamespaceType("Music:Song:PhilosopherSong")
    Music:Song
*/

func NamespaceType(namespace string) string {
	parts := strings.Split(namespace, ":")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[0:len(parts)-1], ":")
}

func NamespaceValue(namespace string) string {
	parts := strings.Split(namespace, ":")
	return parts[len(parts)-1]
}
