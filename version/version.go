package version

import (
	"fmt"

	"github.com/blang/semver"
)

type Formatter interface{}

type Version struct {
	Branch      string
	Commit      string
	ShortCommit string
	Semver      semver.Version
	Revision    int64
}

func New(conf Config) (*Version, error) {
	gen, err := NewGenerator(conf)
	if err != nil {
		return nil, err
	}

	return gen.Generate(conf)
}

func (v *Version) Print() {
	fmt.Printf(
		"Branch: %s\nCommit: %s\nShortCommit: %s\nVersion: %s\nRevision: %d\n",
		v.Branch,
		v.Commit,
		v.ShortCommit,
		v.Semver,
		v.Revision,
	)
}
