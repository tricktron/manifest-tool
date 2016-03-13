STACKUP - A tool to create List Manifests 
=
stackup [![Build Status](https://travis-ci.org/runcom/skopeo.svg?branch=master)](https://travis-ci.org/runcom/skopeo)
=

_Please be aware `stackup` is still work in progress_

`stackup` is a command line utility to create `list manifests`.


Example:
```sh
./stackup /home/harshal/listm.yml
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

License
-
ASL 2.0
