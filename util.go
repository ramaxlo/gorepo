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
	log "github.com/sirupsen/logrus"
)

var ProjectRoot string
var ConfDir string

func SetProjectRoot(isInit bool) {
	if isInit {
		ProjectRoot, _ = os.Getwd()
	} else {
		var err error
		if ProjectRoot, err = findProjectRoot(); err != nil {
			log.Fatalf("%s", err)
		}
	}
	ConfDir = filepath.Join(ProjectRoot, ".gorepo")
}

func findProjectRoot() (prjRoot string, err error) {
	dir, _ := os.Getwd()

	for dir != "/" {
		tmp := filepath.Join(dir, ".gorepo")
		if isDir(tmp) {
			prjRoot = dir
			return
		}

		dir = filepath.Clean(filepath.Join(dir, ".."))
	}

	err = fmt.Errorf("No project root is found")
	return
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
