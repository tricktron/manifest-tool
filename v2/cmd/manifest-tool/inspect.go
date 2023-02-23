package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/docker/distribution/reference"
	"github.com/estesp/manifest-tool/v2/pkg/registry"
	"github.com/estesp/manifest-tool/v2/pkg/store"
	"github.com/estesp/manifest-tool/v2/pkg/types"
	"github.com/estesp/manifest-tool/v2/pkg/util"

	"github.com/fatih/color"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
)

var inspectCmd = &cli.Command{
	Name:  "inspect",
	Usage: "fetch image manifests in a container registry",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "raw",
			Usage: "raw JSON output",
		},
		&cli.BoolFlag{
			Name:  "tags",
			Usage: "include RepoTags in raw response",
			Value: true,
		},
	},
	Action: func(c *cli.Context) error {

		name := c.Args().First()
		imageRef, err := util.ParseName(name)
		if err != nil {
			logrus.Fatal(err)
		}
		if _, ok := imageRef.(reference.NamedTagged); !ok {
			logrus.Fatal("image reference must include a tag; manifest-tool does not default to 'latest'")
		}

		memoryStore := store.NewMemoryStore()
		resolver := util.NewResolver(c.String("username"), c.String("password"), c.Bool("insecure"),
			c.Bool("plain-http"), c.String("docker-cfg"))

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
			return nil
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

		return nil
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

	outputStr := strings.Builder{}
	var attestations int
	for i, img := range index.Manifests {
		var attestationDetail string

		if aRefType, ok := img.Annotations["vnd.docker.reference.type"]; ok {
			if aRefType == "attestation-manifest" {
				attestations++
				attestationDetail = " (vnd.docker.reference.type=attestation-manifest)"
			}
		}
		outputStr.WriteString(fmt.Sprintf("[%d]     Type: %s%s\n", i+1, green(img.MediaType), green(attestationDetail)))
		outputStr.WriteString(fmt.Sprintf("[%d]   Digest: %s\n", i+1, yellow(img.Digest)))
		outputStr.WriteString(fmt.Sprintf("[%d]   Length: %s\n", i+1, blue(img.Size)))

		_, db, _ := cs.Get(img)
		switch img.MediaType {
		case ocispec.MediaTypeImageManifest, types.MediaTypeDockerSchema2Manifest:
			var man ocispec.Manifest
			if err := json.Unmarshal(db, &man); err != nil {
				logrus.Fatal(err)
			}
			if len(attestationDetail) > 0 {
				// only output info about the attestation info
				attestRef := img.Annotations["vnd.docker.reference.digest"]
				outputStr.WriteString(fmt.Sprintf("[%d]       >>> Attestation for digest: %s\n\n", i+1, yellow(attestRef)))
				continue
			}
			outputStr.WriteString(fmt.Sprintf("[%d] Platform:\n", i+1))
			outputStr.WriteString(fmt.Sprintf("[%d]    -      OS: %s\n", i+1, green(img.Platform.OS)))
			if img.Platform.OSVersion != "" {
				outputStr.WriteString(fmt.Sprintf("[%d]    - OS Vers: %s\n", i+1, green(img.Platform.OSVersion)))
			}
			if len(img.Platform.OSFeatures) > 0 {
				outputStr.WriteString(fmt.Sprintf("[%d]    - OS Feat: %s\n", i+1, green(img.Platform.OSFeatures)))
			}
			outputStr.WriteString(fmt.Sprintf("[%d]    -    Arch: %s\n", i+1, green(img.Platform.Architecture)))
			if img.Platform.Variant != "" {
				outputStr.WriteString(fmt.Sprintf("[%d]    - Variant: %s\n", i+1, green(img.Platform.Variant)))
			}
			outputStr.WriteString(fmt.Sprintf("[%d] # Layers: %s\n", i+1, red(len(man.Layers))))
			for j, layer := range man.Layers {
				outputStr.WriteString(fmt.Sprintf("     layer %s: digest = %s\n", red(fmt.Sprintf("%02d", j+1)), yellow(layer.Digest)))
				outputStr.WriteString(fmt.Sprintf("                 type = %s\n", green(layer.MediaType)))
			}
			outputStr.WriteString("\n")
		default:
			outputStr.WriteString(fmt.Sprintf("Unknown media type for further display: %s\n", img.MediaType))
		}
	}
	imageCount := len(index.Manifests) - attestations
	imageStr := "image"
	attestStr := "attestation"
	if imageCount > 1 {
		imageStr = "images"
	}
	if attestations > 1 {
		attestStr = "attestations"
	}
	fmt.Printf(" * Contains %s manifest references (%s %s, %s %s):\n", red(len(index.Manifests)),
		red(imageCount), imageStr, red(attestations), attestStr)
	fmt.Printf("%s", outputStr.String())
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
