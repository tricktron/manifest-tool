#!/usr/bin/env bats


setup_file() {
    load 'common-setup'
    _common_setup
    # create certs
    export HOSTNM=${TEST_REGISTRY_HOST:-myregistry.local}
    mkdir certs
    openssl req -x509 -newkey rsa:4096 -sha256 -days 1 -nodes \
     -keyout certs/ssc.key -out certs/ssc.crt -subj "/CN=${HOSTNM}" \
     -addext "subjectAltName=DNS:${HOSTNM}"

    # start a container registry
    "${RUNTIME_TOOL}" run -d --name registry  \
     -v "$(pwd)"/certs:/certs \
     -e REGISTRY_HTTP_ADDR=0.0.0.0:443 \
     -e REGISTRY_HTTP_TLS_CERTIFICATE=/certs/ssc.crt \
     -e REGISTRY_HTTP_TLS_KEY=/certs/ssc.key \
     -p 443:443 \
     registry:2

    _load_test_images
}

teardown_file() {
    # stop our registry
    "${RUNTIME_TOOL}" stop registry
    "${RUNTIME_TOOL}" rm registry
    rm -fR certs
}

@test "can run manifest-tool version" {
    ./manifest-tool --version
}

@test "can inspect a basic image" {
    ./manifest-tool --insecure inspect ${HOSTNM}/alpine:amd64
}
