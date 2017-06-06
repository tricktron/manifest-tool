## manifest-tool - A tool to query/create manifest list objects in the Docker Registry v2.3 and above

`manifest-tool` is a command line utility to create **manifests list** objects in the Docker registry.
Manifest lists are defined in the [v2.2 image specification](https://github.com/docker/distribution/blob/master/docs/spec/manifest-v2-2.md) and allow for multi-architecture and/or
multi-OS images to be stored in the Docker registry.

### Sample Usage

*Note:* For pushing to an authenticated registry like DockerHub, you will need a config generated via
`docker login`:
```sh
docker login
<enter your credentials>
```

If you are pushing to a registry requiring authentication, you can provide the resulting docker
login configuration when pushing a manifest list:

```sh
./manifest-tool --docker-cfg '/home/myuser/.docker/' push from-spec /home/myuser/sample.yml
```

In the latest version, the user of `manifest-tool` has the option to use either command line
arguments or a YAML file to provide the specified images/tags and platform specifications and
options to use when creating a manifest list within the registry.

A sample YAML file is shown below.  Cross-repository push is exploited in `manifest-tool`
so the source and target image names can differ as long as they are within the same registry.
For example, a source image could be named `myprivreg:5000/someimage_ppc64le:latest` and 
used when creating a manifest list with image target name/tag `myprivreg:5000/someimage:latest`.

With a private registry running on port 5000, a sample YAML input to create a manifest list
combining a ppc64le and amd64 image would look like this:
```
image: myprivreg:5000/someimage:latest
manifests:
  -
    image: myprivreg:5000/someimage:ppc64le
    platform:
      architecture: ppc64le
      os: linux
  -
    image: myprivreg:5000/someimage:amd64
    platform:
      architecture: amd64
      features:
        - sse
      os: linux
```

If your Docker client config is found but does not contain the necessary credentials for the queried registry
you'll receive an error. You can fix this by either logging in (via `docker login`) or providing `--username`
and `--password`.

### Building
-
To build `manifest-tool` use the latest Go version. If using a system that is not at least at the Go
1.6 release level, you will need to export the variable `GO15VENDOREXPERIMENT` into your build environment.
Either set up your `$GOPATH` properly, or clone this repository into your `$GOPATH` to allow compilation to
successfully build the tool.

```sh
$ cd $GOPATH/src
$ mkdir -p github.com/estesp
$ cd github.com/estesp
$ git clone https://github.com/estesp/manifest-tool
$ cd manifest-tool && make binary
```

### Installing

If you built from source:
```sh
$ sudo make install
```

### Tests

**You need Docker installed on your system in order to run the test suite.**

```sh
$ make test-integration
```

### License

Apache License 2.0
