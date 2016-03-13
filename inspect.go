package main

import (
//	"fmt"
	"strings"

	"github.com/codegangsta/cli"
	"github.com/harche/stackup/docker"
//	"github.com/harche/stackup/types"
)

type imgKind int

const (
	imgTypeDocker = "docker://"
	imgTypeAppc   = "appc://"

	kindUnknown = iota
	kindDocker
	kindAppc
)


func inspect(c *cli.Context)  {

	name  := c.Args().First()

	docker.PutData(c, strings.Replace(name, imgTypeDocker, "", -1))

}
