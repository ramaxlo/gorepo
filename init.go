package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/urfave/cli/v2"
)

var CmdInit = cli.Command{
	Name:  "init",
	Usage: "Initailize repositories",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "url",
			Usage:    "The URL of manifest repository",
			Aliases:  []string{"u"},
			Required: true,
		},
		&cli.StringFlag{
			Name:    "manifest",
			Usage:   "The xml file path",
			Value:   "default.xml",
			Aliases: []string{"m"},
		},
		&cli.StringFlag{
			Name:    "branch",
			Usage:   "The branch of the manifest repository",
			Value:   "main",
			Aliases: []string{"b"},
		},
		&cli.BoolFlag{
			Name:  "dump",
			Usage: "Dump content of parsed xml file",
		},
		&cli.BoolFlag{
			Name:  "force-delete",
			Usage: "Delete local manifest repository if it exists",
		},
	},
	Action: cmdInit,
	Before: func(c *cli.Context) error {
		SetProjectRoot(true)
		return nil
	},
}

func dumpManifest(m *Manifest) {
	fmt.Printf("=== Remotes ===\n")
	for _, r := range m.Remotes {
		fmt.Printf("%v\n", r)
	}

	fmt.Printf("=== Defaults ===\n")
	fmt.Printf("%v\n", m.Defaults)

	fmt.Printf("=== Projects ===\n")
	for _, r := range m.Projects {
		fmt.Printf("%v\n", r)
	}
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	if err != nil {
		return false
	}

	if !fi.Mode().IsDir() {
		return false
	}

	return true
}

func initManifest(url, branch string, forceDelete bool) error {
	if !isDir(ConfDir) {
		os.MkdirAll(ConfDir, 0755)
	}

	manifestDir := filepath.Join(ConfDir, "manifests")
	if isDir(manifestDir) {
		if forceDelete {
			os.RemoveAll(manifestDir)
		} else {
			return fmt.Errorf("There is existing local manifest ('.gorepo/manifests'). Please use '--force-delete' to delete it before cloning")
		}
	}

	_, err := git.PlainClone(manifestDir, false, &git.CloneOptions{
		URL:           url,
		ReferenceName: plumbing.ReferenceName(branch),
		Progress:      os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("Fail to clone manifest: %s", err)
	}

	return nil
}

func cmdInit(ctx *cli.Context) error {
	err := initManifest(ctx.String("url"), ctx.String("branch"), ctx.Bool("force-delete"))
	if err != nil {
		return fmt.Errorf("Fail to init manifest repo: %s", err)
	}

	// Check the validity of XML file in manifest repo
	fileName := ctx.String("manifest")
	filePath := filepath.Join(ConfDir, "manifests", fileName)
	m, err := LoadManifest(filePath)
	if err != nil {
		return fmt.Errorf("Fail to parse manifest: %s", err)
	}

	if ctx.Bool("dump") {
		dumpManifest(m)
	}

	cfg := Config{
		Manifest: ManifestInfo{
			Path:   "manifests",
			File:   fileName,
			Branch: ctx.String("branch"),
		},
	}
	err = SaveConfig(&cfg)
	if err != nil {
		return fmt.Errorf("Fail to save config: %s", err)
	}

	return nil
}
