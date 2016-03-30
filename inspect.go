package main

import (
	"strings"

	"github.com/codegangsta/cli"
	"github.com/estesp/manifest-tool/docker"
)

type imgKind int

const (
	imgTypeDocker = "docker://"
	imgTypeAppc   = "appc://"

	kindUnknown = iota
	kindDocker
	kindAppc
)

func inspect(c *cli.Context) {

	name := c.Args().First()

	docker.PutData(c, strings.Replace(name, imgTypeDocker, "", -1))

}
