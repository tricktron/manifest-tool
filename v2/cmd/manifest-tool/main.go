package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/docker/cli/config"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

// filled in at compile time
var gitCommit = ""

const (
	version = "2.0.8"
	usage   = "registry client to inspect and push multi-platform OCI & Docker v2 images"
)

func main() {
	if err := runApplication(); err != nil {
		logrus.Errorf("manifest-tool failed with error: %v", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func runApplication() error {
	app := cli.NewApp()
	app.Name = os.Args[0]
	app.Version = version + " (commit: " + gitCommit + ")"
	app.Usage = usage
	app.Flags = []cli.Flag{
		&cli.BoolFlag{
			Name:  "debug",
			Usage: "enable debug output",
		},
		&cli.BoolFlag{
			Name:  "insecure",
			Usage: "allow insecure registry communication",
		},
		&cli.BoolFlag{
			Name:  "plain-http",
			Usage: "allow registry communication over plain http",
		},
		&cli.StringFlag{
			Name:  "username",
			Value: "",
			Usage: "registry username",
		},
		&cli.StringFlag{
			Name:  "password",
			Value: "",
			Usage: "registry password",
		},
		&cli.StringFlag{
			Name:  "docker-cfg",
			Value: config.Dir(),
			Usage: "either a directory path containing a Docker-formatted config.json or a specific JSON file formatted for registry auth",
		},
	}
	app.Before = func(c *cli.Context) error {
		if c.Bool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		} else {
			logrus.SetLevel(logrus.WarnLevel)
		}
		dockerAuthPath := c.String("docker-cfg")
		// if set to the default, we don't check for validity because it may not
		// even exist
		if dockerAuthPath == config.Dir() {
			if err := c.Set("docker-cfg", filepath.Join(dockerAuthPath, "config.json")); err != nil {
				return fmt.Errorf("unable to update docker-cfg flag in context: %w", err)
			}
			return nil
		}
		// check if the user passed in a directory or an actual file
		// if a dir, then append "config.json" for compatibility; otherwise pass through
		f, err := os.Stat(dockerAuthPath)
		if err != nil {
			return fmt.Errorf("failed to check state of docker-cfg value: %w", err)
		}
		if f.IsDir() {
			if err := c.Set("docker-cfg", filepath.Join(dockerAuthPath, "config.json")); err != nil {
				return fmt.Errorf("unable to set client to context: %w", err)
			}
		}
		return nil
	}
	// currently support inspect and pushml
	app.Commands = []*cli.Command{
		inspectCmd,
		pushCmd,
	}

	return app.Run(os.Args)
}
