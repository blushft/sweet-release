package cmd

import (
	"log"

	"github.com/blushft/sweet-release/version"
	"github.com/spf13/cobra"
)

var (
	repoPath   string
	yearFactor = 1000
)

var rootCommand = &cobra.Command{
	Use:   "sweet-release",
	Short: "Sweet Release automates versioning and releaseing your software",
	RunE:  run,
}

func init() {
	flgs := rootCommand.PersistentFlags()

	flgs.StringVarP(&repoPath, "repo-path", "r", "./", "Path to the root of the project repository")
}

func Execute() error {
	return rootCommand.Execute()
}

func run(cmd *cobra.Command, args []string) error {
	log.Println("starting sweet release")

	conf := version.DefaultConfig()
	conf.RepoPath = repoPath

	ver, err := version.New(conf)
	if err != nil {
		return err
	}

	ver.Print()

	return nil
}
