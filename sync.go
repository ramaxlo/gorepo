package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var CmdSync = cli.Command{
	Name:  "sync",
	Usage: "Update repositories",
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:        "tasks",
			Usage:       "How many tasks are created for updating",
			Value:       0,
			DefaultText: "1",
			Aliases:     []string{"j"},
		},
		&cli.BoolFlag{
			Name:  "force-sync",
			Usage: "Force updating repos",
		},
	},
	Action: cmdSync,
	Before: func(c *cli.Context) error {
		SetProjectRoot(false)
		return nil
	},
}

func syncManifest(cfg *Config) error {
	mlog := log.WithFields(log.Fields{
		"cmd":   "sync",
		"stage": "manifest-sync",
	})
	repoPath := filepath.Join(ConfDir, cfg.Manifest.Path)
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return fmt.Errorf("Fail to open manifest repo: %s", err)
	}

	err = repo.Fetch(&git.FetchOptions{
		Progress: os.Stdout,
	})
	if err != nil {
		if !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return fmt.Errorf("Fail to fetch update: %s", err)
		} else {
			mlog.Info("Manifest up-to-date")
		}
	}

	newBranchNeeded := false
	remoteHash, err := resolveRevision(repo, "origin", cfg.Manifest.Branch)
	if err != nil {
		return fmt.Errorf("Fail to resolve revision: %s", err)
	}

	localRef, err := findBranch(repo, cfg.Manifest.Branch)
	if err == nil {
		mlog.Debugf("localRef: %s, remoteHash: %s", localRef.Hash().String(), remoteHash.String())
		if localRef.Hash().String() != remoteHash.String() {
			mlog.Debug("Remove local branch")
			repo.Storer.RemoveReference(localRef.Name())
			newBranchNeeded = true
		}
	} else {
		mlog.Debugf("%s", err)
		newBranchNeeded = true
	}

	if newBranchNeeded {
		localBranch := fmt.Sprintf("refs/heads/%s", cfg.Manifest.Branch)
		w, _ := repo.Worktree()

		err = w.Checkout(&git.CheckoutOptions{
			Hash:   remoteHash,
			Branch: plumbing.ReferenceName(localBranch),
			Create: true,
		})
		if err != nil {
			return fmt.Errorf("Fail to checkout worktree: %s", err)
		}
	}

	return nil
}

func cmdSync(ctx *cli.Context) error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("Fail to load config: %s", err)
	}

	err = syncManifest(cfg)
	if err != nil {
		return fmt.Errorf("Fail to sync manifest: %s", err)
	}

	filePath := filepath.Join(ConfDir, cfg.Manifest.Path, cfg.Manifest.File)
	m, err := LoadManifest(filePath)
	if err != nil {
		return fmt.Errorf("Fail to load manifest: %s", err)
	}

	n := ctx.Int("tasks")
	// If -j option is not specified, look for 'sync-j' attr in manifest
	if n <= 0 {
		n, _ = m.GetSyncJ()
		if n <= 0 {
			n = 1
		}
	}

	force := ctx.Bool("force-sync")
	err = syncRepos(m, n, force)
	if err != nil {
		return fmt.Errorf("Fail to init repos: %s", err)
	}

	return nil
}

type syncJob struct {
	repo      string
	revision  string
	path      string
	remote    string
	err       error
	log       *log.Entry
	force     bool
	copyFiles []Copyfile
	linkFiles []Linkfile
}

func findBranch(repo *git.Repository, name string) (*plumbing.Reference, error) {
	refs, _ := repo.References()
	var found *plumbing.Reference

	branchName := fmt.Sprintf("refs/heads/%s", name)
	refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference {
			return nil
		}

		refName := ref.Name()
		if string(refName) == branchName {
			found = ref
			return nil
		}

		return nil
	})

	if found == nil {
		return nil, fmt.Errorf("Branch '%s' not found", name)
	}

	return found, nil
}

func pullUpdate(path string, j syncJob) error {
	jlog := j.log

	jlog.Info("Pull update")
	repo, err := git.PlainOpen(path)
	if err != nil {
		return fmt.Errorf("Fail to open git repo: %s", err)
	}

	err = repo.Fetch(&git.FetchOptions{
		RemoteName: j.remote,
		Progress:   os.Stdout,
	})
	if err != nil {
		if !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return fmt.Errorf("Fail to fetch update: %s", err)
		} else {
			jlog.Info("Remote up-to-date")
		}
	}

	remoteHash, err := parseRevision(repo, j.revision, j)
	if err != nil {
		return fmt.Errorf("Fail to parse revision: %s", err)
	}

	newBranchNeeded := false
	localRef, err := findBranch(repo, "manifest-rev")
	if err == nil {
		jlog.Debugf("localRef: %s, remoteHash: %s", localRef.Hash().String(), remoteHash.String())
		if localRef.Hash().String() != remoteHash.String() {
			jlog.Debug("Remove local branch")
			repo.Storer.RemoveReference(localRef.Name())
			newBranchNeeded = true
		}
	} else {
		jlog.Debugf("%s", err)
		newBranchNeeded = true
	}

	//TODO: Create new branch
	if newBranchNeeded {
		w, _ := repo.Worktree()

		// Create new branch 'manifest-rev' pointing to the target revision
		err = w.Checkout(&git.CheckoutOptions{
			Hash:   remoteHash,
			Branch: plumbing.ReferenceName("refs/heads/manifest-rev"),
			Create: true,
		})
		if err != nil {
			return fmt.Errorf("Fail to checkout worktree: %s", err)
		}

		// Checkout in detached mode
		err = w.Checkout(&git.CheckoutOptions{
			Hash:  remoteHash,
			Force: true,
		})
		if err != nil {
			return fmt.Errorf("Fail to checkout worktree: %s", err)
		}
	}

	return nil
}

func parseRevision(repo *git.Repository, revStr string, j syncJob) (plumbing.Hash, error) {
	return resolveRevision(repo, j.remote, revStr)
}

func cloneRepo(path string, j syncJob) error {
	jlog := j.log

	jlog.Info("Clone repo")
	repo, err := git.PlainInit(path, false)
	if err != nil {
		return fmt.Errorf("Fail to init new repo: %s", err)
	}

	jlog.Debug("create remote")
	_, err = repo.CreateRemote(&config.RemoteConfig{
		Name: j.remote,
		URLs: []string{j.repo},
	})
	if err != nil {
		return fmt.Errorf("Fail to create new remote: %s", err)
	}

	jlog.Debug("fetch remote")
	err = repo.Fetch(&git.FetchOptions{
		RemoteName: j.remote,
		Progress:   os.Stdout,
	})
	if err != nil {
		return fmt.Errorf("Fail to fetch update: %s", err)
	}

	jlog.Debug("create branch")
	w, _ := repo.Worktree()
	h, err := parseRevision(repo, j.revision, j)
	if err != nil {
		return fmt.Errorf("Fail to parse revision: %s", err)
	}

	// Create new branch 'manifest-rev' pointing to the target revision
	err = w.Checkout(&git.CheckoutOptions{
		Hash:   h,
		Branch: plumbing.ReferenceName("refs/heads/manifest-rev"),
		Create: true,
	})
	if err != nil {
		return fmt.Errorf("Fail to checkout worktree: %s", err)
	}

	// Checkout in detached mode
	err = w.Checkout(&git.CheckoutOptions{
		Hash: h,
	})
	if err != nil {
		return fmt.Errorf("Fail to checkout worktree: %s", err)
	}

	return nil
}

func doCopy(src, dest string) error {
	sf, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("Fail to open file: %s", err)
	}
	defer sf.Close()

	df, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("Fail to create file: %s", err)
	}
	defer df.Close()

	_, err = io.Copy(df, sf)
	if err != nil {
		return fmt.Errorf("Fail to copy: %s", err)
	}

	return nil
}

func isFile(p string) bool {
	fi, err := os.Stat(p)
	if err != nil {
		return false
	}

	if !fi.Mode().IsRegular() {
		return false
	}

	return true
}

func doLinkfile(repoPath string, l Linkfile, llog *log.Entry) error {
	if l.Src == "" {
		return fmt.Errorf("linkfile src is empty")
	}

	if l.Dest == "" {
		return fmt.Errorf("linkfile dest is empty")
	}

	if filepath.IsAbs(l.Src) {
		return fmt.Errorf("linkfile src is not relative path: %s", l.Src)
	}

	src := filepath.Join(repoPath, l.Src)
	src = filepath.Clean(src)
	if !strings.HasPrefix(src, repoPath) {
		return fmt.Errorf("linkfile src (%s) is outside the repo: %s", l.Src, repoPath)
	}

	if filepath.IsAbs(l.Dest) {
		return fmt.Errorf("linkfile dest is not relative path: %s", l.Dest)
	}

	dest := filepath.Join(ProjectRoot, l.Dest)
	dest = filepath.Clean(dest)
	if !strings.HasPrefix(dest, ProjectRoot) {
		return fmt.Errorf("linkfile dest (%s) is outside the project root", l.Dest)
	}

	if _, err := os.Stat(dest); err == nil {
		llog.Debugf("linkfile dest (%s) exists. Skip.", dest)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	relSrc, err := filepath.Rel(ProjectRoot, src)
	if err != nil {
		return err
	}
	llog.Debugf("Linkfile %s -> %s", relSrc, dest)
	if err := os.Symlink(relSrc, dest); err != nil {
		return err
	}

	return nil
}

func doCopyfile(repoPath string, c Copyfile, clog *log.Entry) error {
	if c.Src == "" {
		return fmt.Errorf("copyfile src is empty")
	}

	if c.Dest == "" {
		return fmt.Errorf("copyfile dest is empty")
	}

	if filepath.IsAbs(c.Src) {
		return fmt.Errorf("copyfile src is not relative path: %s", c.Src)
	}

	src := filepath.Join(repoPath, c.Src)
	src = filepath.Clean(src)
	if !isFile(src) {
		return fmt.Errorf("copyfile src is not a file: %s", c.Src)
	}
	if !strings.HasPrefix(src, repoPath) {
		return fmt.Errorf("copyfile src (%s) is outside of the repo: %s", c.Src, repoPath)
	}

	if filepath.IsAbs(c.Dest) {
		return fmt.Errorf("copyfile dest is not relative path: %s", c.Dest)
	}

	dest := filepath.Join(ProjectRoot, c.Dest)
	dest = filepath.Clean(dest)
	if !strings.HasPrefix(dest, ProjectRoot) {
		return fmt.Errorf("copyfile dest (%s) is outside of the project root", c.Dest)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	clog.Debugf("Copyfile %s -> %s", src, dest)
	if err := doCopy(src, dest); err != nil {
		return err
	}

	return nil
}

func isRemoteDifferent(repoPath string, job syncJob) bool {
	jlog := job.log
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		jlog.Errorf("Fail to open git repo: %s", err)
		return false
	}

	remotes, err := repo.Remotes()
	if err != nil {
		jlog.Errorf("Can not read the remotes: %s", err)
		return false
	}

	isDifferent := true
	for _, r := range remotes {
		remoteURL := r.Config().URLs[0]
		jobURL := job.repo

		jlog.Debugf("remoteURL: %s", remoteURL)
		jlog.Debugf("jobURL: %s", jobURL)
		if remoteURL == jobURL {
			isDifferent = false
		}
	}

	return isDifferent
}

func reCreateDir(repoPath string) (err error) {
	err = os.RemoveAll(repoPath)
	if err != nil {
		return
	}
	err = os.MkdirAll(repoPath, 0755)

	return
}

func doJob(j syncJob) error {
	jlog := j.log
	repoPath := j.path
	if !filepath.IsAbs(repoPath) {
		repoPath = filepath.Join(ProjectRoot, repoPath)
	}

	var err error
	if isDir(repoPath) {
		if isRemoteDifferent(repoPath, j) {
			if !j.force {
				buf := bytes.NewBuffer(nil)
				fmt.Fprintf(buf, "The repo %s has different remote. ", j.path)
				fmt.Fprintf(buf, "Use --force-sync to force the updating.")

				return errors.New(buf.String())
			}

			jlog.Infof("The repo %s has different remote. Force update.", j.path)
			err = reCreateDir(repoPath)
			if err != nil {
				return err
			}
			err = cloneRepo(repoPath, j)
		} else {
			err = pullUpdate(repoPath, j)
		}
	} else {
		err = cloneRepo(repoPath, j)
	}
	if err != nil {
		return err
	}

	for _, c := range j.copyFiles {
		if err := doCopyfile(repoPath, c, jlog); err != nil {
			return err
		}
	}

	for _, l := range j.linkFiles {
		if err := doLinkfile(repoPath, l, jlog); err != nil {
			return err
		}
	}

	return nil
}

func worker(idx int, stopCh <-chan bool, jobCh <-chan syncJob, errCh chan<- syncJob, wg *sync.WaitGroup, logger *log.Entry) {
	wlog := logger.WithFields(log.Fields{
		"worker": idx,
	})
	defer wg.Done()

	for {
		select {
		case <-stopCh:
			wlog.Debug("exit")
			return
		case j := <-jobCh:
			jlog := wlog.WithFields(log.Fields{
				"path":   j.path,
				"remote": j.remote,
			})
			jlog.Debugf("Repo: %s", j.repo)

			j.log = jlog
			start := time.Now()
			err := doJob(j)
			dur := time.Since(start).Round(time.Second)
			if err != nil {
				jlog.Errorf("Fail to do job (dur %s): %s", dur, err)
			} else {
				jlog.Infof("Job done (dur %s)", dur)
			}

			j.err = err
			errCh <- j
		}
	}
}

func createJob(m *Manifest, p *Project) (syncJob, error) {
	name, url, err := m.GetRemote(p)
	if err != nil {
		return syncJob{}, err
	}

	rev, err := m.GetRevision(p)
	if err != nil {
		return syncJob{}, err
	}

	tmp := syncJob{
		repo:      url,
		revision:  rev,
		path:      p.Path,
		remote:    name,
		copyFiles: p.Copyfiles,
		linkFiles: p.Linkfiles,
	}

	return tmp, nil
}

func setupDirAll(j syncJob) {
	dir := filepath.Dir(j.path)
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(ProjectRoot, dir)
	}

	if !isDir(dir) {
		os.MkdirAll(dir, 0755)
	}
}

func syncRepos(m *Manifest, numTasks int, force bool) error {
	slog := log.WithFields(log.Fields{
		"cmd": "sync",
	})
	stopCh := make(chan bool)
	jobCh := make(chan syncJob)
	errCh := make(chan syncJob)
	var wg sync.WaitGroup
	for i := 0; i < numTasks; i++ {
		go worker(i, stopCh, jobCh, errCh, &wg, slog)
		wg.Add(1)
	}
	defer wg.Wait()

	// Job dispatch
	go func() {
		for _, p := range m.Projects {
			job, err := createJob(m, &p)
			if err != nil {
				slog.Debugf("Skip the job %s: %s", p.Name, err)
				continue
			}

			setupDirAll(job)
			job.force = force
			jobCh <- job
		}
	}()

	// Fetch result of processing
	hasError := false
	for i := 0; i < len(m.Projects); i++ {
		j := <-errCh
		if j.err != nil {
			slog.Errorf("Job %s failed", j.path)
			hasError = true
		}
	}

	close(stopCh)

	if hasError {
		return fmt.Errorf("Error happens")
	}

	return nil
}
