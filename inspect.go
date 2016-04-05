package main

import (
	"encoding/json"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/estesp/manifest-tool/docker"
)

var inspectCmd = cli.Command{
	Name:  "inspect",
	Usage: "inspect images on a registry",
	Action: func(c *cli.Context) {

		imgInspect, err := docker.GetData(c, c.Args().First())
		if err != nil {
			logrus.Fatal(err)
		}
		out, err := json.Marshal(imgInspect)
		if err != nil {
			logrus.Fatal(err)
		}
		fmt.Println(string(out))
	},
}
