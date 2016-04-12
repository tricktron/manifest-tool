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
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "raw",
			Usage: "raw JSON output",
		},
	},
	Action: func(c *cli.Context) {

		name := c.Args().First()
		imgInspect, err := docker.GetData(c, name)
		if err != nil {
			logrus.Fatal(err)
		}
		if c.Bool("raw") {
			out, err := json.Marshal(imgInspect)
			if err != nil {
				logrus.Fatal(err)
			}
			fmt.Println(string(out))
			return
		}
		// output basic informative details about the image
		if len(imgInspect) == 1 {
			// this is a basic single manifest
			fmt.Printf("%s: manifest type: %s\n", name, imgInspect[0].MediaType)
			fmt.Printf("      Digest: %s\n", imgInspect[0].Digest)
			fmt.Printf("Architecture: %s\n", imgInspect[0].Architecture)
			fmt.Printf("          OS: %s\n", imgInspect[0].Os)
			return
		}
		// more than one response--this is a manifest list
		fmt.Printf("%s is a manifest list containing the following %d manifest references:\n", name, len(imgInspect))
		for i, img := range imgInspect {
			fmt.Printf("%d    Mfst Type: %s\n", i+1, img.MediaType)
			fmt.Printf("%d       Digest: %s\n", i+1, img.Digest)
			fmt.Printf("%d  Mfst Length: %d\n", i+1, img.Size)
			fmt.Printf("%d Architecture: %s\n", i+1, img.Architecture)
			fmt.Printf("%d           OS: %s\n", i+1, img.Os)
		}
	},
}
