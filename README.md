STACKUP - A tool to create List Manifests 
=
stackup [![Build Status](https://travis-ci.org/runcom/skopeo.svg?branch=master)](https://travis-ci.org/runcom/skopeo)
=

_Please be aware `stackup` is still work in progress_

`stackup` is a command line utility to create `list manifests`.


Example:
If you don't have docker config file already generated then generate it using,
```sh
docker login
```
Docker config file is required for authentication with the repository
```sh
./stackup --docker-cfg '/home/harshal/.docker/' /home/harshal/listm.yml
```
Sample YAML:
```sh
--- 
image: pharshal/myListManifest:latest
manifests: 
  - 
    image: docker.io/fedora:rawhide
    platform: 
      architecture: ppc64
      os: Linux
      variant: ppc64le
  - 
    image: docker.io/ubuntu:latest
    platform: 
      architecture: x86_64
      features: 
        - sse
      os: Linux
```

If your cli config is found but it doesn't contain the necessary credentials for the queried registry
you'll get an error. You can fix this by either logging in (via `docker login`) or providing `--username`
and `--password`.
Building
-
To build `stackup` you need at least Go 1.5 because it uses the latest `GO15VENDOREXPERIMENT` flag. Also, make sure to clone the repository in your `GOPATH` - otherwise compilation fails.
```sh
$ cd $GOPATH/src
$ mkdir -p github.com/harche
$ cd harche
$ git clone https://github.com/harche/stackup
$ cd stackup && make binary
```
Man:
-
To build the man page you need [`go-md2man`](https://github.com/cpuguy83/go-md2man) available on your system, then:
```
$ make man
```
Installing
-
If you built from source:
```sh
$ sudo make install
```
`stackup` is also available from Fedora 23:
```sh
sudo dnf install stackup
```
Tests
-
_You need Docker installed on your system in order to run the test suite_
```sh
$ make test-integration
```
TODO
-
Automatically fill OS and Architechture from digest instead of user putting it in YAML manually

License
-
ASL 2.0
