package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/hash"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var CmdSync = cli.Command{
	Name:  "sync",
	Usage: "Update repositories",
	Flags: []cli.Flag{
		&cli.IntFlag{
			Name:    "tasks",
			Usage:   "How many tasks are created for updating",
			Value:   1,
			Aliases: []string{"j"},
		},
	},
	Action: cmdSync,
}

func cmdSync(ctx *cli.Context) error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("Fail to load config: %s", err)
	}

	filePath := filepath.Join(ConfDir, cfg.Manifest.Path, cfg.Manifest.File)
	m, err := loadManifest(filePath)
	if err != nil {
		return fmt.Errorf("Fail to load manifest: %s", err)
	}

	n := ctx.Int("tasks")
	err = syncRepos(m, n)
	if err != nil {
		return fmt.Errorf("Fail to init repos: %s", err)
	}

	return nil
}

type syncJob struct {
	repo     string
	revision string
	path     string
	remote   string
	err      error
	log      *log.Entry
}

func findBranch(repo *git.Repository, name string) (*plumbing.Reference, error) {
	refs, _ := repo.References()
	var found *plumbing.Reference

	refs.ForEach(func(ref *plumbing.Reference) error {
		if ref.Type() != plumbing.HashReference {
			return nil
		}

		refName := ref.Name()
		if strings.HasSuffix(string(refName), name) {
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
		jlog.Errorf("%s", err)
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
	var h plumbing.Hash
	var err error
	var b []byte

	if b, err = hex.DecodeString(revStr); err == nil {
		if len(b) == hash.HexSize {
			h = plumbing.NewHash(revStr)
		} else {
			return plumbing.Hash{}, fmt.Errorf("Invalid hash format: %s", revStr)
		}
	} else if strings.HasPrefix(revStr, "refs/tags") {
		tmp, err := repo.ResolveRevision(plumbing.Revision(revStr))
		if err != nil {
			return plumbing.Hash{}, fmt.Errorf("Invalid tag: %s", revStr)
		}
		h = *tmp
	} else {
		// DEFAULT CASE
		// If all rules are not matched, then we assume the string to be a branch
		// name of a remote.
		fullRevStr := fmt.Sprintf("refs/remotes/%s/%s", j.remote, revStr)
		//fmt.Printf("ref: %s\n", fullRevStr)
		tmp, err := repo.ResolveRevision(plumbing.Revision(fullRevStr))
		if err != nil {
			return plumbing.Hash{}, fmt.Errorf("Invalid remote branch: %s", fullRevStr)
		}
		h = *tmp
	}

	return h, nil
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

func doJob(j syncJob) error {
	repoPath := j.path
	if !filepath.IsAbs(repoPath) {
		repoPath = filepath.Join(ProjectRoot, repoPath)
	}

	if isDir(repoPath) {
		return pullUpdate(repoPath, j)
	} else {
		return cloneRepo(repoPath, j)
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

func findRemote(m *Manifest, p *Project) (string, string, error) {
	remoteName := p.Remote
	if remoteName == "" {
		remoteName = m.Defaults.Remote
	}
	if remoteName == "" {
		return "", "", fmt.Errorf("No remote is specified, nor default remote name is found")
	}

	for _, r := range m.Remotes {
		if r.Name == remoteName {
			r, _ := url.JoinPath(r.Fetch, p.Name)
			return remoteName, r, nil
		}
	}

	return "", "", fmt.Errorf("No specified remote is found")
}

func findRevision(m *Manifest, p *Project) (string, error) {
	rev := p.Revision
	if rev == "" {
		rev = m.Defaults.Revision
	}
	if rev == "" {
		return "", fmt.Errorf("No revision is specified, nor default revision is found")
	}

	return rev, nil
}

func createJob(m *Manifest, p *Project) (syncJob, error) {
	name, url, err := findRemote(m, p)
	if err != nil {
		return syncJob{}, err
	}

	rev, err := findRevision(m, p)
	if err != nil {
		return syncJob{}, err
	}

	tmp := syncJob{
		repo:     url,
		revision: rev,
		path:     p.Path,
		remote:   name,
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

func syncRepos(m *Manifest, numTasks int) error {
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
			jobCh <- job
		}
	}()

	// Fetch result of processing
	for i := 0; i < len(m.Projects); i++ {
		j := <-errCh
		if j.err != nil {
			slog.Errorf("Job %s failed", j.path)
		}
	}

	close(stopCh)

	return nil
}
