package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/estesp/manifest-tool/docker"
	"github.com/estesp/manifest-tool/types"
)

type imgKind int

const (
	imgTypeDocker = "docker://"
	imgTypeAppc   = "appc://"

	kindUnknown = iota
	kindDocker
	kindAppc
)

func getImgType(img string) imgKind {
	if strings.HasPrefix(img, imgTypeDocker) {
		return kindDocker
	}
	if strings.HasPrefix(img, imgTypeAppc) {
		return kindAppc
	}
	return kindDocker
}

var inspectCmd = cli.Command{
	Name:  "inspect",
	Usage: "inspect images on a registry",
	Action: func(c *cli.Context) {

		imgInspect, err := inspect(c)
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

func inspect(c *cli.Context) (*types.ImageInspect, error) {
	var (
		imgInspect *types.ImageInspect
		err        error
		name       = c.Args().First()
		kind       = getImgType(name)
	)

	switch kind {
	case kindDocker:
		imgInspect, err = docker.GetData(c, strings.Replace(name, imgTypeDocker, "", -1))
		if err != nil {
			return nil, err
		}
	case kindAppc:
		return nil, fmt.Errorf("appc image inspect not yet implemented")
	default:
		return nil, fmt.Errorf("%s image is invalid, please use either 'docker://' or 'appc://'", name)
	}
	return imgInspect, nil
}
