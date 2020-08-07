package version

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/blang/semver/v4"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const (
	secondsYear int64 = 60 * 60 * 24 * 365
)

type Config struct {
	RepoPath       string
	Clone          bool
	Revision       string
	TimeMultiplier int64
	AddSnapshot    bool
	VersionFile    string
	FromFile       bool
	FromTag        bool
	StableBranches []string
}

type Generator struct {
	Conf   Config
	repo   *git.Repository
	status *git.Status

	Branch            string
	SemVer            *semver.Version
	CurrentCommit     *plumbing.Hash
	VersionCommit     *plumbing.Hash
	InitialCommit     *plumbing.Hash
	InitialCommitDate time.Time
	CurrentCommitDate time.Time
	VersionCommitDate time.Time
	CommitCount       int64
	IsSnapshot        bool
	IsPre             bool
	PreChannel        string
	PreRev            int64
}

func NewGenerator(conf Config) (*Generator, error) {
	gen := &Generator{
		Conf: conf,
	}

	repo, err := gen.getRepo()
	if err != nil {
		return nil, err
	}

	gen.repo = repo

	wt, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	status, err := wt.Status()
	if err != nil {
		return nil, err
	}

	gen.status = &status

	if !status.IsClean() {
		if !conf.AddSnapshot {
			return nil, errors.New("repository is not clean and add snapshot config is false")
		}

		gen.IsSnapshot = true
	}

	if err := gen.getBranch(); err != nil {
		return nil, err
	}

	if err := gen.getCurrentCommit(); err != nil {
		return nil, err
	}

	if err := gen.getInitialCommit(); err != nil {
		return nil, err
	}

	if err := gen.getVersion(); err != nil {
		return nil, err
	}

	return gen, nil
}

func (gen *Generator) Generate() (*Version, error) {
	cdiff := gen.CurrentCommitDate.Sub(gen.InitialCommitDate).Seconds()

	tf := int64(cdiff) * gen.Conf.TimeMultiplier / secondsYear
	rev := gen.CommitCount + tf

	ssha := gen.CurrentCommit.String()[:7]

	if gen.IsPre {
		pre, err := semver.NewPRVersion(gen.Branch)
		if err != nil {
			return nil, err
		}

		gen.SemVer.Pre = append(gen.SemVer.Pre, pre)
	}

	build := []string{
		"rev",
		fmt.Sprintf("%d", rev),
	}

	if gen.IsSnapshot {
		snap, err := semver.NewBuildVersion("SNAPSHOT")
		if err != nil {
			return nil, err
		}

		scnt, err := gen.getCommitCountFrom(gen.VersionCommitDate)
		if err != nil {
			return nil, err
		}

		build = append(build, snap, fmt.Sprintf("%d", scnt))
	}

	gen.SemVer.Build = build

	return &Version{
		Branch:      gen.Branch,
		Commit:      gen.CurrentCommit.String(),
		ShortCommit: ssha,
		Semver:      *gen.SemVer,
		BuildID:     rev,
	}, nil
}

func (gen *Generator) getRepo() (*git.Repository, error) {
	return git.PlainOpen(gen.Conf.RepoPath)
}

func (gen *Generator) getBranch() error {
	rev, err := gen.repo.Head()
	if err != nil {
		return err
	}

	gen.Branch = rev.Name().Short()

	gen.IsPre = true
	for _, s := range gen.Conf.StableBranches {
		if strings.EqualFold(gen.Branch, s) {
			gen.IsPre = false
			break
		}
	}

	return nil
}

func (gen *Generator) getCurrentCommit() error {
	cch, err := gen.repo.ResolveRevision(plumbing.Revision(gen.Conf.Revision))
	if err != nil {
		return err
	}

	cc, err := gen.repo.CommitObject(*cch)
	if err != nil {
		return err
	}

	gen.CurrentCommit = cch
	gen.CurrentCommitDate = cc.Committer.When.UTC()

	return err
}

func (gen *Generator) getInitialCommit() error {
	rlog, err := gen.repo.Log(&git.LogOptions{
		Order: git.LogOrderCommitterTime,
		All:   true,
	})
	if err != nil {
		return err
	}

	ccnt := int64(0)

	rlog.ForEach(func(c *object.Commit) error {
		if c.NumParents() == 0 {
			gen.InitialCommit = &c.Hash
			gen.InitialCommitDate = c.Committer.When.UTC()
		}

		ccnt++
		return nil
	})

	gen.CommitCount = ccnt

	return nil
}

func (gen *Generator) getCommitCountFrom(from time.Time) (int64, error) {
	clog, err := gen.repo.Log(&git.LogOptions{
		Since: &from,
	})

	if err != nil {
		return 0, err
	}

	var cnt int64 = 0
	clog.ForEach(func(c *object.Commit) error {
		cnt++
		return nil
	})

	return cnt, nil
}

func (gen *Generator) getVersion() error {
	if err := gen.getVersionFile(); err != nil {
		if gen.Conf.FromFile {
			return err
		}
	}

	if gen.SemVer != nil && !gen.Conf.FromTag {
		return nil
	}

	return gen.getVersionByTag()
}

func (gen *Generator) getVersionFile() error {
	fp := filepath.Join(gen.Conf.RepoPath, gen.Conf.VersionFile)

	b, err := ioutil.ReadFile(fp)
	if os.IsNotExist(err) {
		return nil
	}

	if err != nil {
		return err
	}

	sv, err := semver.ParseTolerant(string(bytes.TrimSpace(b)))
	if err != nil {
		return err
	}

	gen.SemVer = &sv

	return nil
}

func (gen *Generator) getCommittedVersionFile() error {
	commit, err := gen.repo.CommitObject(*gen.CurrentCommit)
	if err != nil {
		return err
	}

	tree, err := commit.Tree()
	if err != nil {
		return err
	}

	for _, f := range tree.Entries {
		if !f.Mode.IsFile() {
			continue
		}

		if f.Name != "VERSION" {
			continue
		}

		obj, err := gen.repo.BlobObject(f.Hash)
		if err != nil {
			log.Println(err)
			continue
		}

		r, err := obj.Reader()
		if err != nil {
			return err
		}

		b, err := ioutil.ReadAll(r)
		if err != nil {
			return err
		}

		ver, err := semver.ParseTolerant(string(b))
		if err != nil {
			return err
		}

		gen.SemVer = &ver
	}

	return nil
}

func (gen *Generator) getVersionByTag() error {
	tags, err := gen.repo.Tags()
	if err != nil {
		return err
	}

	defer tags.Close()

	var cctag string
	svts := make(map[plumbing.Hash]semver.Version)

	cont := true

	for cont {
		tag, err := tags.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			continue
		}

		if tag.Hash() == *gen.CurrentCommit {
			cctag = tag.Name().Short()
			cont = false

			continue
		}

		ver, err := semver.ParseTolerant(tag.Name().Short())
		if err == nil {
			svts[tag.Hash()] = ver
		}
	}

	if len(cctag) > 0 {
		sv, err := semver.ParseTolerant(cctag)
		if err == nil {
			gen.VersionCommit = gen.CurrentCommit
			gen.VersionCommitDate = gen.CurrentCommitDate
			gen.SemVer = &sv
			return nil
		}
	}

	if gen.Conf.FromTag && !gen.Conf.AddSnapshot {
		return errors.New("could not find tag for commit " + gen.CurrentCommit.String())
	}

	gen.IsSnapshot = true

	if len(svts) == 0 {
		if gen.Conf.FromTag {
			return errors.New("could not find version tag for SNAPSHOT")
		}
	}

	var latest *semver.Version
	var vh plumbing.Hash
	for h, t := range svts {
		if latest == nil {
			vh = h
			latest = &t
		}

		if t.GT(*latest) {
			vh = h
			latest = &t
		}
	}

	vc, err := gen.repo.CommitObject(vh)
	if err != nil {
		return err
	}

	gen.VersionCommit = &vh
	gen.VersionCommitDate = vc.Committer.When.UTC()
	gen.SemVer = latest

	return nil
}
