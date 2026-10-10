package main

import (
	"io/fs"
)

// frontendFS serves frontend/ at the root, so index.html is at /
func frontendFS() fs.FS {
	sub, err := fs.Sub(assets, "frontend")
	if err != nil {
		panic(err)
	}
	return sub
}
