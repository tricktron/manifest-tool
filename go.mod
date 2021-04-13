module github.com/estesp/manifest-tool

go 1.16

require (
	github.com/containerd/containerd v1.5.0-rc.1
	github.com/deislabs/oras v0.8.1
	github.com/docker/cli v20.10.0-beta1+incompatible // indirect
	github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker v1.4.2-0.20200203170920-46ec8731fbce
	github.com/docker/go-connections v0.4.1-0.20190612165340-fd1b1942c4d5 // indirect
	github.com/fatih/color v1.10.0
	github.com/gorilla/mux v1.7.4-0.20190830121156-884b5ffcbd3a // indirect
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.0.1
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.7.0
	github.com/urfave/cli v1.22.2
	gopkg.in/yaml.v2 v2.4.0
	rsc.io/letsencrypt v0.0.3 // indirect
)

replace github.com/docker/docker => github.com/moby/moby v0.7.3-0.20190826074503-38ab9da00309
