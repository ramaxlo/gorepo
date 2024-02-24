package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/jedib0t/go-pretty/v6/table"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var CmdInfo = cli.Command{
	Name:   "info",
	Usage:  "List info of repositories",
	Action: cmdInfo,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "show-url",
			Usage: "Print URLs of repositories",
		},
	},
}

func cmdInfo(ctx *cli.Context) error {
	ilog := log.WithFields(log.Fields{
		"cmd": "info",
	})
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("Fail to load config: %s", err)
	}

	filePath := filepath.Join(ConfDir, cfg.Manifest.Path, cfg.Manifest.File)
	m, err := loadManifest(filePath)
	if err != nil {
		return fmt.Errorf("Fail to load manifest: %s", err)
	}

	showUrl := ctx.Bool("show-url")
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	if showUrl {
		t.AppendHeader(table.Row{"Path", "Current revision", "Manifest revision", "Url"})
	} else {
		t.AppendHeader(table.Row{"Path", "Current revision", "Manifest revision"})
	}

	manifestRepo := filepath.Join(ConfDir, cfg.Manifest.Path)
	err = manifestInfo(t, manifestRepo)
	if err != nil {
		return fmt.Errorf("Fail to list manifest info: %s", err)
	}

	err = repoInfo(t, m, ilog, showUrl)
	if err != nil {
		return fmt.Errorf("Fail to list repo info: %s", err)
	}

	return nil
}

func manifestInfo(t table.Writer, repoPath string) error {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("Fail to open manifest repo: %s", err)
	}

	rev, err := repo.ResolveRevision(plumbing.Revision("HEAD"))
	if err != nil {
		return fmt.Errorf("Fail to resolve manifest revision: %s", err)
	}

	t.AppendRow(table.Row{
		filepath.Base(repoPath),
		rev.String(),
	})
	t.AppendSeparator()

	return nil
}

func repoInfo(t table.Writer, m *Manifest, ilog *log.Entry, showUrl bool) error {
	for _, p := range m.Projects {
		plog := ilog.WithFields(log.Fields{
			"project": p.Path,
		})
		curRev, manifestRev, err := getRevs(m, &p)
		if err != nil {
			plog.Errorf("Fail to get rev: %s", err)
			continue
		}

		ilog.Debugf("%s, %s", curRev, manifestRev)
		if showUrl {
			_, url, _ := findRemote(m, &p)
			t.AppendRow(table.Row{
				p.Path,
				curRev,
				manifestRev,
				url,
			})
		} else {
			t.AppendRow(table.Row{
				p.Path,
				curRev,
				manifestRev,
			})
		}
		//t.AppendSeparator()
	}

	t.AppendFooter(table.Row{"Total", len(m.Projects)})
	t.Render()

	return nil
}

func getRevs(m *Manifest, p *Project) (curRev, manifestRev string, err error) {
	relPath := p.Path
	repoPath := filepath.Join(ProjectRoot, relPath)

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		err = fmt.Errorf("Fail to open repo: %s", err)
		return
	}

	rev, err := repo.ResolveRevision(plumbing.Revision("refs/heads/manifest-rev"))
	if err != nil {
		err = fmt.Errorf("Fail to resolve revision: %s", err)
		return
	}

	curRev = rev.String()
	manifestRev, err = findRevision(m, p)
	if err != nil {
		err = fmt.Errorf("Fail to read revision: %s", err)
		return
	}

	return
}
