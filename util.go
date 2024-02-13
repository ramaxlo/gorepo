package main

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
)

var ProjectRoot string
var ConfDir string

func init() {
	ProjectRoot, _ = os.Getwd()
	ConfDir = filepath.Join(ProjectRoot, ".gorepo")
}

func loadManifest(filePath string) (manifest Manifest, err error) {
	f, err := os.Open(filePath)
	if err != nil {
		err = fmt.Errorf("Fail to open file: %s", err)
		return
	}
	defer f.Close()

	decoder := xml.NewDecoder(f)

	err = decoder.Decode(&manifest)
	if err != nil {
		err = fmt.Errorf("Fail to parse xml: %s", err)
		return
	}

	return
}
