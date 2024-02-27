package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/hash"
)

var ProjectRoot string
var ConfDir string

func init() {
	ProjectRoot, _ = os.Getwd()
	ConfDir = filepath.Join(ProjectRoot, ".gorepo")
}

func resolveRevision(repo *git.Repository, remote, revStr string) (out plumbing.Hash, err error) {
	var b []byte
	var tmp *plumbing.Hash

	if b, err = hex.DecodeString(revStr); err == nil {
		if len(b) == hash.Size {
			out = plumbing.NewHash(revStr)
		} else {
			err = fmt.Errorf("Invalid hash format: %s", revStr)
		}
	} else if strings.HasPrefix(revStr, "refs/tags") {
		tmp, err = repo.ResolveRevision(plumbing.Revision(revStr))
		if err != nil {
			err = fmt.Errorf("Invalid tag: %s", revStr)
			return
		}
		out = *tmp
	} else {
		// DEFAULT CASE
		// If all rules are not matched, then we assume the string to be a branch
		// name of a remote.
		fullRevStr := fmt.Sprintf("refs/remotes/%s/%s", remote, revStr)
		//fmt.Printf("ref: %s\n", fullRevStr)
		tmp, err = repo.ResolveRevision(plumbing.Revision(fullRevStr))
		if err != nil {
			err = fmt.Errorf("Invalid remote branch: %s", fullRevStr)
			return
		}
		out = *tmp
	}

	return
}
