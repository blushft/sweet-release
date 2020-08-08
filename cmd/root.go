package cmd

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/blushft/sweet-release/template"
	"github.com/blushft/sweet-release/version"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCommand = &cobra.Command{
	Use:              "sweet-release",
	Short:            "Sweet Release automates versioning and releasing your software",
	PersistentPreRun: configure,
	RunE:             run,
}

func init() {
	flgs := rootCommand.PersistentFlags()

	flgs.StringP("repo-path", "r", "./", "Path to the root of the project repository")
	flgs.Bool("clone", false, "Clone the repo if path points to url")
	flgs.StringP("semver", "v", "", "Supply the semver by flag")
	flgs.StringP("config-file", "c", "", "Configure with file")
	flgs.Bool("from-file", false, "Get version from file")
	flgs.String("version-file", "VERSION", "File name containing semver")
	flgs.Bool("from-tag", false, "Get version from git tag")
	flgs.String("revision", "HEAD", "Git revision or branch")
	flgs.Int64("time-multiplier", 1000, "Time multiplier to generate atomic build nubmer")
	flgs.Bool("add-snapshot", false, "Add snapshot pre to semver")
	flgs.StringSlice("stable-branches", []string{}, "Branches to treat as stable")

	flgs.String("out", "version_file", "Desired output")

	viper.BindPFlags(flgs)
	viper.SetEnvPrefix("SREL")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	viper.SetDefault("repo-path", "./")
	viper.SetDefault("clone", false)
	viper.SetDefault("revision", "HEAD")
	viper.SetDefault("time-multiplier", 1000)
	viper.SetDefault("add-snapshot", false)
	viper.SetDefault("version-file", "VERSION")
	viper.SetDefault("from-file", false)
	viper.SetDefault("from-tag", true)
	viper.SetDefault("stable-branches", []string{"master", "main", "stable", "release"})
	viper.SetDefault("out", "version_file")
}

func Execute() error {
	return rootCommand.Execute()
}

func configure(cmd *cobra.Command, args []string) {

	configFile := viper.GetString("config-file")
	if configFile != "" {
		if err := loadConfigFile(configFile); err != nil {
			panic(err)
		}
	}
}

func loadConfigFile(f string) error {
	ext := filepath.Ext(f)
	if ext == "" || ext == f {
		viper.SetConfigType("json")

	}

	viper.AddConfigPath("./")
	viper.SetConfigFile(f)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil
		}

		if _, ok := err.(*os.PathError); ok {
			return nil
		}

		return err
	}

	return nil
}

func run(cmd *cobra.Command, args []string) error {
	conf := getConfig()

	ver, err := version.New(conf)
	if err != nil {
		return err
	}

	tmpl, err := template.New()

	if err := tmpl.Execute(viper.GetString("out"), ver); err != nil {
		return err
	}

	return nil
}

func getConfig() version.Config {
	return version.Config{
		RepoPath:       viper.GetString("repo-path"),
		Clone:          viper.GetBool("clone"),
		Revision:       viper.GetString("revision"),
		TimeMultiplier: viper.GetInt64("time-multiplier"),
		AddSnapshot:    viper.GetBool("add-snapshot"),
		FromTag:        viper.GetBool("from-tag"),
		VersionFile:    viper.GetString("version-file"),
		FromFile:       viper.GetBool("from-file"),
		StableBranches: viper.GetStringSlice("stable-branches"),
	}
}
