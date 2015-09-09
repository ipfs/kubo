package mfs

import (
	"errors"
	"fmt"
	"strings"
)

func rootLookup(r *Root, path string) (FSNode, error) {
	dir, ok := r.GetValue().(*Directory)
	if !ok {
		return nil, errors.New("root was not a directory")
	}

	return DirLookup(dir, path)
}

// DirLookup will look up a file or directory at the given path
// under the directory 'd'
func DirLookup(d *Directory, path string) (FSNode, error) {
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 1 && parts[0] == "" {
		return d, nil
	}

	var cur FSNode
	cur = d
	for i, p := range parts {
		chdir, ok := cur.(*Directory)
		if !ok {
			return nil, fmt.Errorf("cannot access %s: Not a directory", strings.Join(parts[:i+1], "/"))
		}

		child, err := chdir.Child(p)
		if err != nil {
			return nil, err
		}

		cur = child
	}
	return cur, nil
}
