package version

import (
	"fmt"

	"github.com/blang/semver/v4"
)

type Formatter interface{}

type Version struct {
	Branch      string         `json:"branch"`
	Commit      string         `json:"commit"`
	ShortCommit string         `json:"short_commit"`
	Semver      semver.Version `json:"semver"`
	BuildID     int64          `json:"build_id"`
}

func New(conf Config) (*Version, error) {
	gen, err := NewGenerator(conf)
	if err != nil {
		return nil, err
	}

	return gen.Generate()
}

func (v *Version) Print() {
	fmt.Printf(
		"Branch: %s\nCommit: %s\nShortCommit: %s\nVersion: %s\nRevision: %d\n",
		v.Branch,
		v.Commit,
		v.ShortCommit,
		v.Semver,
		v.BuildID,
	)
}
