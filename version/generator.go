package version

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"time"

	"github.com/blang/semver"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const (
	secondsYear int64 = 60 * 60 * 24 * 365
)

type Config struct {
	RepoPath          string `json:"repo_path" yaml:"repo_path" toml:"repo_path"`
	Clone             bool   `json:"clone" yaml:"clone" toml:"clone"`
	Revision          string `json:"base_branch" yaml:"base_branch" toml:"base_branch"`
	YearFactor        int64  `json:"year_factor" yaml:"year_factor" toml:"year_factor"`
	AddSnapshot       bool   `json:"add_snapshot" yaml:"add_snapshot" toml:"add_snapshot"`
	RequireVersionTag bool   `json:"require_version_tag" yaml:"require_version_tag" toml:"require_version_tag"`
}

func DefaultConfig() Config {
	return Config{
		RepoPath:          "./",
		Clone:             false,
		Revision:          "HEAD",
		YearFactor:        1000,
		AddSnapshot:       true,
		RequireVersionTag: true,
	}
}

type Generator struct {
	conf Config

	Branch            string
	CurrentCommit     *plumbing.Hash
	VersionCommit     *plumbing.Hash
	CommitCount       int64
	SnapshotCount     int64
	SemVer            *semver.Version
	InitialCommitDate time.Time
	CurrentCommitDate time.Time
}

func NewGenerator(conf Config) (*Generator, error) {
	gen := &Generator{
		conf: conf,
	}

	repo, err := gen.getRepo()
	if err != nil {
		return nil, err
	}

	if err := gen.getBranch(repo); err != nil {
		return nil, err
	}

	if err := gen.getCurrentCommit(repo); err != nil {
		return nil, err
	}

	if err := gen.getInitialCommit(repo); err != nil {
		return nil, err
	}

	if err := gen.getVersion(repo); err != nil {
		return nil, err
	}

	return gen, nil
}

func (g *Generator) Generate(conf Config) (*Version, error) {
	cdiff := g.CurrentCommitDate.Sub(g.InitialCommitDate).Seconds()

	tf := int64(cdiff) * conf.YearFactor / secondsYear
	rev := g.CommitCount + tf

	ssha := g.CurrentCommit.String()[:7]

	return &Version{
		Branch:      g.Branch,
		Commit:      g.CurrentCommit.String(),
		ShortCommit: ssha,
		Semver:      *g.SemVer,
		Revision:    rev,
	}, nil
}

func (gen *Generator) getRepo() (*git.Repository, error) {
	return git.PlainOpen(gen.conf.RepoPath)
}

func (gen *Generator) getBranch(repo *git.Repository) error {
	rev, err := repo.Head()
	if err != nil {
		return err
	}

	gen.Branch = rev.Name().Short()

	return nil
}

func (gen *Generator) getCurrentCommit(repo *git.Repository) error {
	cch, err := repo.ResolveRevision(plumbing.Revision(gen.conf.Revision))
	if err != nil {
		return err
	}

	cc, err := repo.CommitObject(*cch)
	if err != nil {
		return err
	}

	gen.CurrentCommit = cch
	gen.CurrentCommitDate = cc.Committer.When.UTC()

	return err
}

func (gen *Generator) getInitialCommit(repo *git.Repository) error {
	rlog, err := repo.Log(&git.LogOptions{
		Order: git.LogOrderCommitterTime,
		All:   true,
	})
	if err != nil {
		return err
	}

	ccnt := int64(0)

	rlog.ForEach(func(c *object.Commit) error {
		if c.NumParents() == 0 {
			gen.InitialCommitDate = c.Committer.When.UTC()
		}

		ccnt++
		return nil
	})

	gen.CommitCount = ccnt

	return nil
}

func (gen *Generator) getVersion(repo *git.Repository) error {
	if err := gen.getVersionFile(repo); err != nil {
		return err
	}

	if gen.SemVer != nil && !gen.conf.RequireVersionTag {
		return nil
	}

	return gen.getVersionByTag(repo)
}

func (gen *Generator) getVersionFile(repo *git.Repository) error {
	commit, err := repo.CommitObject(*gen.CurrentCommit)
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

		obj, err := repo.BlobObject(f.Hash)
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

func (gen *Generator) getVersionByTag(repo *git.Repository) error {
	tags, err := repo.Tags()
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
			gen.SemVer = &sv
			return nil
		}
	}

	if gen.conf.RequireVersionTag && !gen.conf.AddSnapshot {
		return errors.New("could not find tag for commit " + gen.CurrentCommit.String())
	}

	if len(svts) == 0 {
		if gen.conf.RequireVersionTag {
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

	gen.VersionCommit = &vh
	gen.SemVer = latest

	return nil
}
