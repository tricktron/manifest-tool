## manifest - A tool to query/create manifest list objects in the Docker Registry v2.3 and above

`manifest` is a command line utility to create **manifests list** objects in the Docker registry.
Manifest lists are defined in the v2.2 image specification and allow for multi-architecture and/or
multi-OS images to be stored in the Docker registry.

### Sample Usage

*Note:* For pushing to an authenticated registry like DockerHub, you will need a config generated via
`docker login`:
```sh
docker login
<enter your credentials>
```

The Docker config file generated from the login is required for authentication with the repository
from the manifest tool:

```sh
./manifest --docker-cfg '/home/myuser/.docker/' /home/myuser/sample.yml
```

In the current version, a YAML file defines the images which will be combined into a manifest list
object. A sample YAML file is shown below.  Note that until `manifest` has cross-repository push
implemented, the images must be in the same repo and only the tag can differ.

Using a private registry running on port 5000, a sample YAML might look like:
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

If your cli config is found but it doesn't contain the necessary credentials for the queried registry
you'll receive an error. You can fix this by either logging in (via `docker login`) or providing `--username`
and `--password`.

### Building
-
To build `manifest` you need either Go 1.6, or Go 1.5 with the variable `GO15VENDOREXPERIMENT` exported.
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

### TODO

 1. Cross-repository push support
 2. Automatically populate OS and architecture from source manifests?

### License

Apache License 2.0
