package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/estesp/manifest-tool/v2/pkg/registry"
	"github.com/estesp/manifest-tool/v2/pkg/store"
	"github.com/estesp/manifest-tool/v2/pkg/types"
	"github.com/estesp/manifest-tool/v2/pkg/util"

	"github.com/fatih/color"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var inspectCmd = cli.Command{
	Name:  "inspect",
	Usage: "fetch image manifests in a container registry",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "raw",
			Usage: "raw JSON output",
		},
		cli.BoolTFlag{
			Name:  "tags",
			Usage: "include RepoTags in raw response",
		},
	},
	Action: func(c *cli.Context) {

		name := c.Args().First()
		imageRef, err := util.ParseName(name)
		if err != nil {
			logrus.Fatal(err)
		}

		memoryStore := store.NewMemoryStore()
		resolver := util.NewResolver(c.GlobalString("username"), c.GlobalString("password"), c.GlobalBool("insecure"),
			c.GlobalBool("plain-http"), filepath.Join(c.GlobalString("docker-cfg"), "config.json"))

		descriptor, err := registry.FetchDescriptor(resolver, memoryStore, imageRef)
		if err != nil {
			logrus.Error(err)
		}

		if c.Bool("raw") {
			out, err := json.Marshal(descriptor)
			if err != nil {
				logrus.Fatal(err)
			}
			fmt.Println(string(out))
			return
		}
		_, db, _ := memoryStore.Get(descriptor)
		switch descriptor.MediaType {
		case ocispec.MediaTypeImageIndex, types.MediaTypeDockerSchema2ManifestList:
			// this is a multi-platform image descriptor; marshal to Index type
			var idx ocispec.Index
			if err := json.Unmarshal(db, &idx); err != nil {
				logrus.Fatal(err)
			}
			outputList(name, memoryStore, descriptor, idx)
		case ocispec.MediaTypeImageManifest, types.MediaTypeDockerSchema2Manifest:
			var man ocispec.Manifest
			if err := json.Unmarshal(db, &man); err != nil {
				logrus.Fatal(err)
			}
			_, cb, _ := memoryStore.Get(man.Config)
			var conf ocispec.Image
			if err := json.Unmarshal(cb, &conf); err != nil {
				logrus.Fatal(err)
			}
			outputImage(name, descriptor, man, conf)
		default:
			logrus.Errorf("Unknown descriptor type: %s", descriptor.MediaType)
		}
	},
}

func outputList(name string, cs *store.MemoryStore, descriptor ocispec.Descriptor, index ocispec.Index) {
	var (
		yellow = color.New(color.Bold, color.FgYellow).SprintFunc()
		red    = color.New(color.Bold, color.FgRed).SprintFunc()
		blue   = color.New(color.Bold, color.FgBlue).SprintFunc()
		green  = color.New(color.Bold, color.FgGreen).SprintFunc()
	)
	fmt.Printf("Name:   %s (Type: %s)\n", green(name), green(descriptor.MediaType))
	fmt.Printf("Digest: %s\n", yellow(descriptor.Digest))
	fmt.Printf(" * Contains %s manifest references:\n", red(len(index.Manifests)))
	for i, img := range index.Manifests {
		fmt.Printf("[%d]     Type: %s\n", i+1, green(img.MediaType))
		fmt.Printf("[%d]   Digest: %s\n", i+1, yellow(img.Digest))
		fmt.Printf("[%d]   Length: %s\n", i+1, blue(img.Size))
		_, db, _ := cs.Get(img)
		switch img.MediaType {
		case ocispec.MediaTypeImageManifest, types.MediaTypeDockerSchema2Manifest:
			var man ocispec.Manifest
			if err := json.Unmarshal(db, &man); err != nil {
				logrus.Fatal(err)
			}
			fmt.Printf("[%d] Platform:\n", i+1)
			fmt.Printf("[%d]    -      OS: %s\n", i+1, green(img.Platform.OS))
			if img.Platform.OSVersion != "" {
				fmt.Printf("[%d]    - OS Vers: %s\n", i+1, green(img.Platform.OSVersion))
			}
			if len(img.Platform.OSFeatures) > 0 {
				fmt.Printf("[%d]    - OS Feat: %s\n", i+1, green(img.Platform.OSFeatures))
			}
			fmt.Printf("[%d]    -    Arch: %s\n", i+1, green(img.Platform.Architecture))
			if img.Platform.Variant != "" {
				fmt.Printf("[%d]    - Variant: %s\n", i+1, green(img.Platform.Variant))
			}
			fmt.Printf("[%d] # Layers: %s\n", i+1, red(len(man.Layers)))
			for j, layer := range man.Layers {
				fmt.Printf("     layer %s: digest = %s\n", red(fmt.Sprintf("%02d", j+1)), yellow(layer.Digest))
			}
			fmt.Println()
		default:
			fmt.Printf("Unknown media type for further display: %s\n", img.MediaType)
		}

	}
}

func outputImage(name string, descriptor ocispec.Descriptor, manifest ocispec.Manifest, config ocispec.Image) {
	var (
		yellow = color.New(color.Bold, color.FgYellow).SprintFunc()
		red    = color.New(color.Bold, color.FgRed).SprintFunc()
		blue   = color.New(color.Bold, color.FgBlue).SprintFunc()
		green  = color.New(color.Bold, color.FgGreen).SprintFunc()
	)
	fmt.Printf("Name: %s (Type: %s)\n", green(name), green(descriptor.MediaType))
	fmt.Printf("      Digest: %s\n", yellow(descriptor.Digest))
	fmt.Printf("        Size: %s\n", blue(descriptor.Size))
	fmt.Printf("          OS: %s\n", green(config.OS))
	fmt.Printf("        Arch: %s\n", green(config.Architecture))
	fmt.Printf("    # Layers: %s\n", red(len(manifest.Layers)))
	for i, layer := range manifest.Layers {
		fmt.Printf("      layer %s: digest = %s\n", red(fmt.Sprintf("%02d", i+1)), yellow(layer.Digest))
	}
}
