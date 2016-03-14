package docker

import (
	"bytes"
//	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
//	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/manifestlist"
	distreference "github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/docker/distribution/registry/api/v2"
	"github.com/docker/distribution/registry/client"
	"github.com/docker/distribution/registry/client/auth"
	"github.com/docker/distribution/registry/client/transport"

	"github.com/docker/docker/api"
	"github.com/docker/docker/cliconfig"
	"github.com/docker/docker/distribution"
	"github.com/docker/docker/dockerversion"
	"github.com/docker/docker/image"
	"github.com/docker/docker/opts"
	versionPkg "github.com/docker/docker/pkg/version"
	"github.com/docker/docker/reference"
	"github.com/docker/docker/registry"
	engineTypes "github.com/docker/engine-api/types"
	registryTypes "github.com/docker/engine-api/types/registry"
	"github.com/harche/stackup/types"
	"golang.org/x/net/context"
)

type existingTokenHandler struct {
	token string
}

type dumbCredentialStore struct {
	auth *engineTypes.AuthConfig
}

func (dcs dumbCredentialStore) Basic(*url.URL) (string, string) {
	return dcs.auth.Username, dcs.auth.Password
}

func (th *existingTokenHandler) Scheme() string {
	return "bearer"
}

func (th *existingTokenHandler) AuthorizeRequest(req *http.Request, params map[string]string) error {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", th.token))
	return nil
}

// fallbackError wraps an error that can possibly allow fallback to a different
// endpoint.
type fallbackError struct {
	// err is the error being wrapped.
	err error
	// confirmedV2 is set to true if it was confirmed that the registry
	// supports the v2 protocol. This is used to limit fallbacks to the v1
	// protocol.
	confirmedV2 bool
	transportOK bool
}

// Error renders the FallbackError as a string.
func (f fallbackError) Error() string {
	return f.err.Error()
}

type manifestFetcher interface {
	Fetch(ctx context.Context, ref reference.Named) (*types.ImageInspect, error)
	Put(c *cli.Context, ctx context.Context, ref reference.Named)
}

func validateName(name string) error {
	distref, err := distreference.ParseNamed(name)
	if err != nil {
		return err
	}
	hostname, _ := distreference.SplitHostname(distref)
	if hostname == "" {
		return fmt.Errorf("Please use a fully qualified repository name")
	}
	return nil
}

func checkHTTPRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}

	if len(via) > 0 {
		for headerName, headerVals := range via[0].Header {
			if headerName != "Accept" && headerName != "Range" {
				continue
			}
			for _, val := range headerVals {
				// Don't add to redirected request if redirected
				// request already has a header with the same
				// name and value.
				hasValue := false
				for _, existingVal := range req.Header[headerName] {
					if existingVal == val {
						hasValue = true
						break
					}
				}
				if !hasValue {
					req.Header.Add(headerName, val)
				}
			}
		}
	}

	return nil
}

func PutData(c *cli.Context, filePath string) {
	fmt.Println("YAML File path - " + filePath)
	filename, err := filepath.Abs(filePath)
	if err != nil {
		panic(err)
	}
	yamlFile, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	//Parse YAML
	var yamlManifestList YAMLManifestList
	err = yaml.Unmarshal(yamlFile, &yamlManifestList)
	if err != nil {
		panic(err)
	}

	var ListManifest manifestlist.ManifestList
	err = yaml.Unmarshal(yamlFile, &ListManifest)
	if err != nil {
		panic(err)
	}

	fmt.Println("Getting the digests of the image names..,")
	for i, img := range yamlManifestList.Manifests {
		imgInsp, err := GetData(c, img.Image)
		if err != nil {
			panic(err)
		}
		imgDigest := imgInsp.Digest
		fmt.Println(img.Image + " : " +imgDigest)
		ListManifest.Manifests[i].Descriptor.Digest, err = digest.ParseDigest(imgDigest)
		if err != nil {
			panic(err)
		}
		ListManifest.Manifests[i].MediaType = imgInsp.MediaType
		ListManifest.Manifests[i].Size = 3355 //hardcoded for testing bug - imgInsp.Size always reports 0
	}

	// Set the version
	ListManifest.Versioned = manifestlist.SchemaVersion

	// Get Named reference for the list manifest
	ref, _ := reference.ParseNamed(yamlManifestList.Image)
	repoInfo, _ := registry.ParseRepositoryInfo(ref)

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
	}
	logrus.Debugf("repoName: %v", repoName)

	// Set the tag to latest, if no tag found in YAML
	if _, isTagged := ref.(reference.NamedTagged); !isTagged {
		ref, _ = reference.WithTag(ref, reference.DefaultTag)

	}
	tagged, _ := ref.(reference.NamedTagged)
	ref, _ = reference.WithTag(ref, tagged.Tag())

	urlBuilder, _ := v2.NewURLBuilderFromString(endpoint.URL.String())
	manifestURL, _ := urlBuilder.BuildManifestURL(ref)
	fmt.Println("Manifest url : - " + manifestURL)

	// deserialized manifest is using during upload
	manifestDescriptors := ListManifest.Manifests
	deserializedManifestList, err := manifestlist.FromDescriptors(manifestDescriptors)
	if err != nil {
		panic(err)
	}
	mediaType, p, err := deserializedManifestList.Payload()
	if err != nil {
		panic(err)

	}
	putRequest, err := http.NewRequest("PUT", manifestURL, bytes.NewReader(p))
	if err != nil {
		panic(err)
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
		//TLSClientConfig: &tls.Config{InsecureSkipVerify: true},

		DisableKeepAlives: true,
	}
	authConfig, err := getAuthConfig(c, repoInfo.Index)
	if err != nil {
		panic(err)
	}
	modifiers := registry.DockerHeaders(dockerversion.DockerUserAgent(), http.Header{})
	authTransport := transport.NewTransport(base, modifiers...)
	challengeManager, _, err := registry.PingV2Registry(endpoint, authTransport)
	if err != nil {
		panic(err) // This seems to be issue while interacting with local registry
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
		// TODO(dmcgowan): create cookie jar
	}

	resp, err := httpClient.Do(putRequest)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	fmt.Println(resp.Status)
	if SuccessStatus(resp.StatusCode) {
		dgstHeader := resp.Header.Get("Docker-Content-Digest")
		dgst, err := digest.ParseDigest(dgstHeader)
		if err != nil {
			panic(err)
		}
		fmt.Println("DIGEST")
		fmt.Println(dgst)
		fmt.Println("DIGEST")

	}

}

func SuccessStatus(status int) bool {
	return status >= 200 && status <= 399
}
func GetData(c *cli.Context, name string) (*types.ImageInspect, error) {
	if err := validateName(name); err != nil {
		return nil, err
	}
	ref, err := reference.ParseNamed(name)
	if err != nil {
		return nil, err
	}
	repoInfo, err := registry.ParseRepositoryInfo(ref)
	if err != nil {
		return nil, err
	}
	authConfig, err := getAuthConfig(c, repoInfo.Index)
	if err != nil {
		return nil, err
	}
	if err := validateRepoName(repoInfo.Name()); err != nil {
		return nil, err
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

	endpoints, err := registryService.LookupPullEndpoints(repoInfo.Hostname())
	if err != nil {
		return nil, err
	}
	logrus.Debugf("endpoints: %v", endpoints)

	var (
		ctx                    = context.Background()
		lastErr                error
		discardNoSupportErrors bool
		imgInspect             *types.ImageInspect
		confirmedV2            bool
		confirmedTLSRegistries = make(map[string]struct{})
	)

	for _, endpoint := range endpoints {
		// make sure I can reach the registry, same as docker pull does

		v1endpoint, err := endpoint.ToV1Endpoint(dockerversion.DockerUserAgent(), nil)
		if err != nil {
			return nil, err
		}
		if _, err := v1endpoint.Ping(); err != nil {
			if strings.Contains(err.Error(), "timeout") {
				return nil, err
			}
			continue
		}

		if confirmedV2 && endpoint.Version == registry.APIVersion1 {
			logrus.Debugf("Skipping v1 endpoint %s because v2 registry was detected", endpoint.URL)
			continue
		}

		if endpoint.URL.Scheme != "https" {
			if _, confirmedTLS := confirmedTLSRegistries[endpoint.URL.Host]; confirmedTLS {
				logrus.Debugf("Skipping non-TLS endpoint %s for host/port that appears to use TLS", endpoint.URL)
				continue
			}
		}

		logrus.Debugf("Trying to fetch image manifest of %s repository from %s %s", repoInfo.Name(), endpoint.URL, endpoint.Version)

		//fetcher, err := newManifestFetcher(endpoint, repoInfo, config)
		fetcher, err := newManifestFetcher(endpoint, repoInfo, authConfig, registryService)
		if err != nil {
			lastErr = err
			continue
		}

		//fetcher.Put(c, ctx, ref)
		if imgInspect, err = fetcher.Fetch(ctx, ref); err != nil {
			// Was this fetch cancelled? If so, don't try to fall back.
			fallback := false
			select {
			case <-ctx.Done():
			default:
				if fallbackErr, ok := err.(fallbackError); ok {
					fallback = true
					confirmedV2 = confirmedV2 || fallbackErr.confirmedV2
					if fallbackErr.transportOK && endpoint.URL.Scheme == "https" {
						confirmedTLSRegistries[endpoint.URL.Host] = struct{}{}
					}
					err = fallbackErr.err
				}
			}
			if fallback {
				if _, ok := err.(distribution.ErrNoSupport); !ok {
					// Because we found an error that's not ErrNoSupport, discard all subsequent ErrNoSupport errors.
					discardNoSupportErrors = true
					// save the current error
					lastErr = err
				} else if !discardNoSupportErrors {
					// Save the ErrNoSupport error, because it's either the first error or all encountered errors
					// were also ErrNoSupport errors.
					lastErr = err
				}
				continue
			}
			logrus.Errorf("Not continuing with pull after error: %v", err)
			return nil, err
		}

		return imgInspect, nil
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no endpoints found for %s", ref.String())
	}

	return nil, lastErr
}

func newManifestFetcher(endpoint registry.APIEndpoint, repoInfo *registry.RepositoryInfo, authConfig engineTypes.AuthConfig, registryService *registry.Service) (manifestFetcher, error) {
	switch endpoint.Version {
	case registry.APIVersion2:
		return &v2ManifestFetcher{
			endpoint:   endpoint,
			authConfig: authConfig,
			service:    registryService,
			repoInfo:   repoInfo,
		}, nil
	case registry.APIVersion1:
		return &v1ManifestFetcher{
			endpoint:   endpoint,
			authConfig: authConfig,
			service:    registryService,
			repoInfo:   repoInfo,
		}, nil
	}
	return nil, fmt.Errorf("unknown version %d for registry %s", endpoint.Version, endpoint.URL)
}

func getAuthConfig(c *cli.Context, index *registryTypes.IndexInfo) (engineTypes.AuthConfig, error) {

	var (
		username      = c.GlobalString("username")
		password      = c.GlobalString("password")
		cfg           = c.GlobalString("docker-cfg")
		defAuthConfig = engineTypes.AuthConfig{
			Username: c.GlobalString("username"),
			Password: c.GlobalString("password"),
			Email:    "stub@example.com",
		}
	)

	//
	// FINAL TODO(runcom): avoid returning empty config! just fallthrough and return
	// the first useful authconfig
	//

	// TODO(runcom): ??? atomic needs this
	// TODO(runcom): implement this to opt-in for docker-cfg, no need to make this
	// work by default with docker's conf
	//useDockerConf := c.GlobalString("use-docker-cfg")

	if username != "" && password != "" {
		return defAuthConfig, nil
	}

	confFile, err := cliconfig.Load(cfg)
	if err != nil {
		return engineTypes.AuthConfig{}, err
	}
	authConfig := registry.ResolveAuthConfig(confFile.AuthConfigs, index)
	logrus.Debugf("authConfig for %s: %v", index.Name, authConfig)

	return authConfig, nil
}

func validateRepoName(name string) error {
	if name == "" {
		return fmt.Errorf("Repository name can't be empty")
	}
	if name == api.NoBaseImageSpecifier {
		return fmt.Errorf("'%s' is a reserved name", api.NoBaseImageSpecifier)
	}
	return nil
}

func makeImageInspect(img *image.Image, tag string, dgst digest.Digest, tagList []string, size int64) *types.ImageInspect {
	var digest string
	if err := dgst.Validate(); err == nil {
		digest = dgst.String()
	}
	return &types.ImageInspect{
		Size:            size,
		Tag:             tag,
		Digest:          digest,
		RepoTags:        tagList,
		Comment:         img.Comment,
		Created:         img.Created.Format(time.RFC3339Nano),
		ContainerConfig: &img.ContainerConfig,
		DockerVersion:   img.DockerVersion,
		Author:          img.Author,
		Config:          img.Config,
		Architecture:    img.Architecture,
		Os:              img.OS,
	}
}

func makeRawConfigFromV1Config(imageJSON []byte, rootfs *image.RootFS, history []image.History) (map[string]*json.RawMessage, error) {
	var dver struct {
		DockerVersion string `json:"docker_version"`
	}

	if err := json.Unmarshal(imageJSON, &dver); err != nil {
		return nil, err
	}

	useFallback := versionPkg.Version(dver.DockerVersion).LessThan("1.8.3")

	if useFallback {
		var v1Image image.V1Image
		err := json.Unmarshal(imageJSON, &v1Image)
		if err != nil {
			return nil, err
		}
		imageJSON, err = json.Marshal(v1Image)
		if err != nil {
			return nil, err
		}
	}

	var c map[string]*json.RawMessage
	if err := json.Unmarshal(imageJSON, &c); err != nil {
		return nil, err
	}

	c["rootfs"] = rawJSON(rootfs)
	c["history"] = rawJSON(history)

	return c, nil
}

func rawJSON(value interface{}) *json.RawMessage {
	jsonval, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	return (*json.RawMessage)(&jsonval)
}

func continueOnError(err error) bool {
	switch v := err.(type) {
	case errcode.Errors:
		if len(v) == 0 {
			return true
		}
		return continueOnError(v[0])
	case distribution.ErrNoSupport:
		return continueOnError(v.Err)
	case errcode.Error:
		return shouldV2Fallback(v)
	case *client.UnexpectedHTTPResponseError:
		return true
	case ImageConfigPullError:
		return false
	case error:
		return !strings.Contains(err.Error(), strings.ToLower(syscall.ENOSPC.Error()))
	}
	// let's be nice and fallback if the error is a completely
	// unexpected one.
	// If new errors have to be handled in some way, please
	// add them to the switch above.
	return true
}

// shouldV2Fallback returns true if this error is a reason to fall back to v1.
func shouldV2Fallback(err errcode.Error) bool {
	switch err.Code {
	case errcode.ErrorCodeUnauthorized, v2.ErrorCodeManifestUnknown, v2.ErrorCodeNameUnknown:
		return true
	}
	return false
}
