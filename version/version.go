package version

import (
	"log"
	"time"

	"github.com/blang/semver"
	"github.com/davecgh/go-spew/spew"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const (
	secondsYear int64 = 60 * 60 * 24 * 365
)

type Formatter interface{}

type Config struct {
	RepoPath               string `json:"repo_path" yaml:"repo_path" toml:"repo_path"`
	Clone                  bool   `json:"clone" yaml:"clone" toml:"clone"`
	Revision               string `json:"base_branch" yaml:"base_branch" toml:"base_branch"`
	YearFactor             int64  `json:"year_factor" yaml:"year_factor" toml:"year_factor"`
	AddSnapshot            bool   `json:"add_snapshot" yaml:"add_snapshot" toml:"add_snapshot"`
	AddLocalChangesDetails bool   `json:"add_local_changes_details" yaml:"add_local_changes_details" toml:"add_local_changes_details"`
}

func DefaultConfig() Config {
	return Config{
		RepoPath:               "./",
		Clone:                  false,
		Revision:               "HEAD",
		YearFactor:             1000,
		AddSnapshot:            false,
		AddLocalChangesDetails: false,
	}
}

type Generator struct {
	Branch            string
	CurrentCommit     *plumbing.Hash
	CommitCount       int64
	SemVer            semver.Version
	InitialCommitDate time.Time
	CurrentCommitDate time.Time
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
		Semver:      g.SemVer,
		Revision:    rev,
	}, nil
}

type Version struct {
	Branch      string
	Commit      string
	ShortCommit string
	Semver      semver.Version
	Revision    int64
}

func New(conf Config) (*Version, error) {
	gen := &Generator{}

	repo, err := getRepo(conf)
	if err != nil {
		return nil, err
	}

	cch, err := repo.ResolveRevision(plumbing.Revision(conf.Revision))
	if err != nil {
		return nil, err
	}

	cc, err := repo.CommitObject(*cch)
	if err != nil {
		return nil, err
	}

	gen.CurrentCommit = cch
	gen.CurrentCommitDate = cc.Committer.When.UTC()

	rev, err := repo.Head()
	if err != nil {
		return nil, err
	}

	gen.Branch = rev.Name().Short()

	rlog, err := repo.Log(&git.LogOptions{
		Order: git.LogOrderCommitterTime,
		All:   true,
	})

	spew.Dump(rlog)

	ccnt := int64(0)

	rlog.ForEach(func(c *object.Commit) error {
		if c.NumParents() == 0 {
			gen.InitialCommitDate = c.Committer.When.UTC()
		}

		ccnt++
		return nil
	})

	gen.CommitCount = ccnt

	tags, err := repo.Tags()
	if err != nil {
		return nil, err
	}

	var ctag string
	tags.ForEach(func(t *plumbing.Reference) error {

		if t.Hash() == *cch {
			log.Println("found tag", t.Name().Short())
			ctag = t.Name().Short()
		}

		return nil
	})

	sv, err := semver.ParseTolerant(ctag)
	if err == nil {
		gen.SemVer = sv
	} else {
		log.Println(err)
	}

	return gen.Generate(conf)
}

func getRepo(conf Config) (*git.Repository, error) {
	return git.PlainOpen(conf.RepoPath)
}
