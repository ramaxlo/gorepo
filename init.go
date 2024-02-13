package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/urfave/cli/v2"
)

type Manifest struct {
	XMLName  string    `xml:"manifest"`
	Defaults Default   `xml:"default"`
	Remotes  []Remote  `xml:"remote"`
	Projects []Project `xml:"project"`
}

type Remote struct {
	Name  string `xml:"name,attr"`
	Fetch string `xml:"fetch,attr"`
}

type Default struct {
	Revision string   `xml:"revision,attr"`
	Remote   string   `xml:"remote,attr"`
	Others   []string `xml:",any,attr"`
}

type Project struct {
	Name     string `xml:"name,attr"`
	Path     string `xml:"path,attr"`
	Remote   string `xml:"remote,attr"`
	Revision string `xml:"revision,attr"`
}

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

	cmd := exec.Command("git", "clone", "-b", branch, url, manifestDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(cmd.Env, "LANG=en")

	err := cmd.Run()
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
	m, err := loadManifest(filePath)
	if err != nil {
		return fmt.Errorf("Fail to parse manifest: %s", err)
	}

	if ctx.Bool("dump") {
		dumpManifest(&m)
	}

	return nil
}
