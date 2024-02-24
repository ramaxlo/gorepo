package main

import (
	"os"
	"path/filepath"
)

var ProjectRoot string
var ConfDir string

func init() {
	ProjectRoot, _ = os.Getwd()
	ConfDir = filepath.Join(ProjectRoot, ".gorepo")
}
