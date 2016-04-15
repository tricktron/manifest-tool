package docker

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/manifestlist"
	"github.com/docker/distribution/registry/api/v2"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"
	"github.com/go-yaml/yaml"

	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
)

// YAMLInput represents the YAML format input to the pushml
// command.
type YAMLInput struct {
	Image     string
	Manifests []ManifestEntry
}

// ManifestEntry represents an entry in the list of manifests to
// be combined into a manifest list, provided via the YAML input
type ManifestEntry struct {
	Image    string
	Platform manifestlist.PlatformSpec
}

func PutManifestList(c *cli.Context, filePath string) (string, error) {
	var (
		yamlInput    YAMLInput
		manifestList manifestlist.ManifestList
	)

	filename, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("Can't resolve path to %q: %v", filePath, err)
	}
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("Can't read YAML file %q: %v", filePath, err)
	}
	err = yaml.Unmarshal(yamlFile, &yamlInput)
	if err != nil {
		return "", fmt.Errorf("Can't unmarshal YAML file %q: %v", filePath, err)
	}

	// Set the schema version
	manifestList.Versioned = manifestlist.SchemaVersion

	logrus.Info("Retrieving digests of images...")
	for _, img := range yamlInput.Manifests {
		// validate os/arch input
		if !isValidOSArch(img.Platform.OS, img.Platform.Architecture) {
			return "", fmt.Errorf("Manifest entry for image %s has unsupported os/arch combination: %s/%s", img.Image, img.Platform.OS, img.Platform.Architecture)
		}
		mfstData, err := GetData(c, img.Image)
		if err != nil {
			return "", fmt.Errorf("Inspect of image %q failed with error: %v", img.Image, err)
		}
		if len(mfstData) > 1 {
			// too many responses--can only happen if a manifest list was returned for the name lookup
			return "", fmt.Errorf("You specified a manifest list entry from a digest that points to a current manifest list. Manifest lists do not allow recursion.")
		}
		// the non-manifest list case will always have exactly one manifest response
		imgMfst := mfstData[0]

		manifest := manifestlist.ManifestDescriptor{
			Platform: img.Platform,
		}
		manifest.Descriptor.Digest, err = digest.ParseDigest(imgMfst.Digest)
		manifest.Size = imgMfst.Size
		manifest.MediaType = imgMfst.MediaType

		if err != nil {
			return "", fmt.Errorf("Digest parse of image %q failed with error: %v", img.Image, err)
		}
		logrus.Infof("Image %q is digest %s; size: %d", img.Image, imgMfst.Digest, imgMfst.Size)
		manifestList.Manifests = append(manifestList.Manifests, manifest)
	}

	// process the final image name reference for the manifest list
	ref, err := reference.ParseNamed(yamlInput.Image)
	if err != nil {
		return "", fmt.Errorf("Error parsing name for manifest list (%s): %v", yamlInput.Image, err)
	}
	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return "", fmt.Errorf("Error parsing repository name for manifest list (%s): %v", yamlInput.Image, err)
	}

	options := &registry.Options{}
	options.Mirrors = opts.NewListOpts(nil)
	options.InsecureRegistries = opts.NewListOpts(nil)
	options.InsecureRegistries.Set("0.0.0.0/0")
	registryService := registry.NewService(options)
	// TODO(runcom): hacky, provide a way of passing tls cert (flag?) to be used to lookup
	for _, ic := range registryService.Config.IndexConfigs {
		ic.Secure = false
	}

	endpoints, _ := registryService.LookupPushEndpoints(repoInfo.Hostname())
	logrus.Debugf("endpoints: %v", endpoints)
	endpoint := endpoints[0]

	repoName := repoInfo.FullName()
	// If endpoint does not support CanonicalName, use the RemoteName instead
	if endpoint.TrimHostname {
		repoName = repoInfo.RemoteName()
		logrus.Debugf("repoName: %v", repoName)
	}
	// get rid of hostname so URL is constructed properly
	hostname, name := splitHostname(ref.String())
	ref, _ = reference.ParseNamed(name)
	logrus.Debugf("hostname: %q; repoName: %q", hostname, name)

	// Set the tag to latest, if no tag found in YAML
	if _, isTagged := ref.(reference.NamedTagged); !isTagged {
		ref, _ = reference.WithTag(ref, reference.DefaultTag)

	}
	tagged, _ := ref.(reference.NamedTagged)
	ref, _ = reference.WithTag(ref, tagged.Tag())

	urlBuilder, _ := v2.NewURLBuilderFromString(endpoint.URL.String())
	manifestURL, _ := urlBuilder.BuildManifestURL(ref)
	logrus.Debugf("Manifest url: %s", manifestURL)

	deserializedManifestList, err := manifestlist.FromDescriptors(manifestList.Manifests)
	if err != nil {
		return "", fmt.Errorf("Cannot deserialize manifest list: %v", err)
	}
	mediaType, p, err := deserializedManifestList.Payload()
	logrus.Debugf("mediaType of manifestList: %s", mediaType)
	if err != nil {
		return "", fmt.Errorf("Cannot retrieve payload for HTTP PUT of manifest list: %v", err)

	}
	putRequest, err := http.NewRequest("PUT", manifestURL, bytes.NewReader(p))
	if err != nil {
		return "", fmt.Errorf("HTTP PUT request creation failed: %v", err)
	}
	putRequest.Header.Set("Content-Type", mediaType)

	// get the http transport, this will be used in a client to upload manifest
	// TODO - add separate function get client
	base := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
		TLSClientConfig:     endpoint.TLSConfig,
		DisableKeepAlives:   true,
	}
	authConfig, err := getAuthConfig(c, repoInfo.Index)
	if err != nil {
		return "", fmt.Errorf("Cannot retrieve authconfig: %v", err)
	}
	modifiers := registry.DockerHeaders(dockerversion.DockerUserAgent(), http.Header{})
	authTransport := transport.NewTransport(base, modifiers...)
	challengeManager, _, err := registry.PingV2Registry(endpoint, authTransport)
	if err != nil {
		return "", fmt.Errorf("Ping of V2 registry failed: %v", err)
	}
	if authConfig.RegistryToken != "" {
		passThruTokenHandler := &existingTokenHandler{token: authConfig.RegistryToken}
		modifiers = append(modifiers, auth.NewAuthorizer(challengeManager, passThruTokenHandler))
	} else {
		creds := dumbCredentialStore{auth: &authConfig}
		tokenHandler := auth.NewTokenHandler(authTransport, creds, repoName, "*")
		basicHandler := auth.NewBasicHandler(creds)
		modifiers = append(modifiers, auth.NewAuthorizer(challengeManager, tokenHandler, basicHandler))
	}
	tr := transport.NewTransport(base, modifiers...)

	httpClient := &http.Client{
		Transport:     tr,
		CheckRedirect: checkHTTPRedirect,
	}

	resp, err := httpClient.Do(putRequest)
	if err != nil {
		return "", fmt.Errorf("V2 registry PUT of manifest list failed: %v", err)
	}

	defer resp.Body.Close()

	if statusSuccess(resp.StatusCode) {
		dgstHeader := resp.Header.Get("Docker-Content-Digest")
		dgst, err := digest.ParseDigest(dgstHeader)
		if err != nil {
			return "", err
		}
		return string(dgst), nil
	}
	return "", fmt.Errorf("Registry push unsuccessful: response %d: %s", resp.StatusCode, resp.Status)

}

func statusSuccess(status int) bool {
	return status >= 200 && status <= 399
}
