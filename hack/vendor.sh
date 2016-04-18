#!/usr/bin/env bash
set -e

cd "$(dirname "$BASH_SOURCE")/.."
rm -rf vendor/
source 'hack/.vendor-helpers.sh'

clone git github.com/codegangsta/cli v1.2.0
clone git github.com/Sirupsen/logrus v0.8.7
clone git github.com/vbatts/tar-split v0.9.11
clone git github.com/gorilla/mux master
clone git github.com/gorilla/context master
clone git golang.org/x/net master https://github.com/golang/net.git
clone git github.com/go-check/check v1
clone git github.com/go-yaml/yaml v2

clone git github.com/docker/docker v1.11.0
clone git github.com/docker/engine-api v0.3.3
clone git github.com/docker/distribution d06d6d3b093302c02a93153ac7b06ebc0ffd1793

clone git github.com/docker/go-connections master
clone git github.com/docker/go-units master
clone git github.com/docker/libtrust master
clone git github.com/opencontainers/runc master

clean

mv vendor/src/* vendor/
