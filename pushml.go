package main

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/estesp/manifest-tool/docker"
)

var pushmlCmd = cli.Command{
	Name:  "pushml",
	Usage: "push a manifest list to a registry via a YAML config",
	Action: func(c *cli.Context) {

		digest, err := pushManifestList(c)
		if err != nil {
			logrus.Fatal(err)
		}
		fmt.Printf("Digest: %s\n", digest)
	},
}

func pushManifestList(c *cli.Context) (string, error) {

	yamlFile := c.Args().First()

	return docker.PutManifestList(c, yamlFile)
}
