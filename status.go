package main

import (
	"fmt"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var CmdStatus = cli.Command{
	Name:   "status",
	Usage:  "List status of repositories",
	Action: cmdStatus,
}

func cmdStatus(ctx *cli.Context) error {
	slog := log.WithFields(log.Fields{
		"cmd": "status",
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

	err = repoStatus(m, slog)
	if err != nil {
		return fmt.Errorf("Fail to list status: %s", err)
	}

	return nil
}

func repoStatus(m *Manifest, slog *log.Entry) error {
	for _, p := range m.Projects {
		relPath := p.Path
		repoPath := filepath.Join(ProjectRoot, relPath)
		printStatus(repoPath, slog)
	}

	return nil
}

func printStatus(repoPath string, slog *log.Entry) error {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("Fail to open repo: %s", err)
	}

	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("Fail to get worktree: %s", err)
	}

	status, err := w.Status()
	if err != nil {
		return fmt.Errorf("Fail to get worktree status: %s", err)
	}

	fmt.Printf("=== Status of %s\n", filepath.Base(repoPath))
	for f, s := range status {
		indicator := getIndicator(s)
		fmt.Printf("  %s  %s\n", indicator, f)
	}

	return nil
}

func getIndicator(s *git.FileStatus) string {
	return fmt.Sprintf("%s%s", getCode(s.Staging), getCode(s.Worktree))
}

func getCode(c git.StatusCode) string {
	if c == git.Unmodified {
		return "-"
	}

	buf := []byte{byte(c)}
	return string(buf)
}
