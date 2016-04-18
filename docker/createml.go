package docker

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
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

type blobMount struct {
	FromRepo string
	Digest   string
}

func PutManifestList(c *cli.Context, filePath string) (string, error) {
	var (
		yamlInput         YAMLInput
		manifestList      manifestlist.ManifestList
		blobMountRequests []blobMount
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

	// process the final image name reference for the manifest list
	targetRef, err := reference.ParseNamed(yamlInput.Image)
	if err != nil {
		return "", fmt.Errorf("Error parsing name for manifest list (%s): %v", yamlInput.Image, err)
	}
	targetRepo, err := registry.ParseRepositoryInfo(targetRef)
	if err != nil {
		return "", fmt.Errorf("Error parsing repository name for manifest list (%s): %v", yamlInput.Image, err)
	}
	targetEndpoint, repoName, err := setupRepo(targetRepo)
	if err != nil {
		return "", fmt.Errorf("Error setting up repository endpoint and references for %q: %v", targetRef, err)
	}

	// Now create the manifest list payload by looking up the manifest schemas
	// for the constituent images:
	logrus.Info("Retrieving digests of images...")
	for _, img := range yamlInput.Manifests {
		// validate os/arch input
		if !isValidOSArch(img.Platform.OS, img.Platform.Architecture) {
			return "", fmt.Errorf("Manifest entry for image %s has unsupported os/arch combination: %s/%s", img.Image, img.Platform.OS, img.Platform.Architecture)
		}
		mfstData, repoInfo, err := GetImageData(c, img.Image)
		if err != nil {
			return "", fmt.Errorf("Inspect of image %q failed with error: %v", img.Image, err)
		}
		if repoInfo.Hostname() != targetRepo.Hostname() {
			return "", fmt.Errorf("Cannot use source images from a different registry than the target image: %s != %s", repoInfo.Hostname(), targetRepo.Hostname())
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

		// if this image is in a different repo, we need to add the layer/blob digests to the list of
		// requested blob mounts (cross-repository push) before pushing the manifest list
		if repoName != repoInfo.RemoteName() {
			logrus.Debugf("Adding layers of %q to blob mount requests", img.Image)
			for _, layer := range imgMfst.Layers {
				blobMountRequests = append(blobMountRequests, blobMount{FromRepo: repoInfo.RemoteName(), Digest: layer})
			}
		}

		manifestList.Manifests = append(manifestList.Manifests, manifest)
	}

	// Set the schema version
	manifestList.Versioned = manifestlist.SchemaVersion

	urlBuilder, err := v2.NewURLBuilderFromString(targetEndpoint.URL.String())
	if err != nil {
		return "", fmt.Errorf("Can't create URL builder from endpoint (%s): %v", targetEndpoint.URL.String(), err)
	}
	pushURL, err := createURLFromTargetRef(targetRef, urlBuilder)
	if err != nil {
		return "", fmt.Errorf("Error setting up repository endpoint and references for %q: %v", targetRef, err)
	}
	logrus.Debugf("Manifest list push url: %s", pushURL)

	deserializedManifestList, err := manifestlist.FromDescriptors(manifestList.Manifests)
	if err != nil {
		return "", fmt.Errorf("Cannot deserialize manifest list: %v", err)
	}
	mediaType, p, err := deserializedManifestList.Payload()
	logrus.Debugf("mediaType of manifestList: %s", mediaType)
	if err != nil {
		return "", fmt.Errorf("Cannot retrieve payload for HTTP PUT of manifest list: %v", err)

	}
	putRequest, err := http.NewRequest("PUT", pushURL, bytes.NewReader(p))
	if err != nil {
		return "", fmt.Errorf("HTTP PUT request creation failed: %v", err)
	}
	putRequest.Header.Set("Content-Type", mediaType)

	httpClient, err := getHTTPClient(c, targetRepo, targetEndpoint, repoName)
	if err != nil {
		return "", fmt.Errorf("Failed to setup HTTP client to repository: %v", err)
	}

	// before we push the manifest list, if we have any blob mount requests, we need
	// to ask the registry to mount those blobs in our target so they are available
	// as references
	if err := mountBlobs(httpClient, urlBuilder, targetRef, blobMountRequests); err != nil {
		return "", fmt.Errorf("Couldn't mount blobs for cross-repository push: %v", err)
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

func getHTTPClient(c *cli.Context, repoInfo *registry.RepositoryInfo, endpoint registry.APIEndpoint, repoName string) (*http.Client, error) {
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
		return nil, fmt.Errorf("Cannot retrieve authconfig: %v", err)
	}
	modifiers := registry.DockerHeaders(dockerversion.DockerUserAgent(nil), http.Header{})
	authTransport := transport.NewTransport(base, modifiers...)
	challengeManager, _, err := registry.PingV2Registry(endpoint, authTransport)
	if err != nil {
		return nil, fmt.Errorf("Ping of V2 registry failed: %v", err)
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
	return httpClient, nil
}

func createURLFromTargetRef(targetRef reference.Named, urlBuilder *v2.URLBuilder) (string, error) {
	// get rid of hostname so the target URL is constructed properly
	_, name := splitHostname(targetRef.String())
	targetRef, err := reference.ParseNamed(name)
	if err != nil {
		return "", fmt.Errorf("Can't parse target image repository name from reference: %v", err)
	}

	// Set the tag to latest, if no tag found in YAML
	if _, isTagged := targetRef.(reference.NamedTagged); !isTagged {
		targetRef, err = reference.WithTag(targetRef, reference.DefaultTag)
		if err != nil {
			return "", fmt.Errorf("Error adding default tag to target repository name: %v", err)
		}
	} else {
		tagged, _ := targetRef.(reference.NamedTagged)
		targetRef, err = reference.WithTag(targetRef, tagged.Tag())
		if err != nil {
			return "", fmt.Errorf("Error referencing specified tag to target repository name: %v", err)
		}
	}

	manifestURL, err := urlBuilder.BuildManifestURL(targetRef)
	if err != nil {
		return "", fmt.Errorf("Failed to build manifest URL from target reference: %v", err)
	}
	return manifestURL, nil
}

func setupRepo(repoInfo *registry.RepositoryInfo) (registry.APIEndpoint, string, error) {

	options := registry.ServiceOptions{}
	options.InsecureRegistries = append(options.InsecureRegistries, "0.0.0.0/0")
	registryService := registry.NewService(options)

	endpoints, err := registryService.LookupPushEndpoints(repoInfo.Hostname())
	if err != nil {
		return registry.APIEndpoint{}, "", err
	}
	logrus.Debugf("endpoints: %v", endpoints)
	// take highest priority endpoint
	endpoint := endpoints[0]

	repoName := repoInfo.FullName()
	// If endpoint does not support CanonicalName, use the RemoteName instead
	if endpoint.TrimHostname {
		repoName = repoInfo.RemoteName()
		logrus.Debugf("repoName: %v", repoName)
	}
	return endpoint, repoName, nil
}

func mountBlobs(httpClient *http.Client, urlBuilder *v2.URLBuilder, ref reference.Named, blobsRequested []blobMount) error {
	// get rid of hostname so the target URL is constructed properly
	_, name := splitHostname(ref.String())
	targetRef, _ := reference.ParseNamed(name)

	for _, blob := range blobsRequested {
		// create URL request
		url, err := urlBuilder.BuildBlobUploadURL(targetRef, url.Values{"from": {blob.FromRepo}, "mount": {blob.Digest}})
		if err != nil {
			return fmt.Errorf("Failed to create blob mount URL: %v", err)
		}
		mountRequest, err := http.NewRequest("POST", url, nil)
		if err != nil {
			return fmt.Errorf("HTTP POST request creation for blob mount failed: %v", err)
		}
		mountRequest.Header.Set("Content-Length", "0")
		resp, err := httpClient.Do(mountRequest)
		if err != nil {
			return fmt.Errorf("V2 registry POST of blob mount failed: %v", err)
		}

		resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("Blob mount failed to url %s: HTTP status %d", url, resp.StatusCode)
		}
		logrus.Debugf("Mount of blob %s succeeded, location: %q", blob.Digest, resp.Header.Get("Location"))
	}
	return nil
}

func statusSuccess(status int) bool {
	return status >= 200 && status <= 399
}
