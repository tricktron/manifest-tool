#!/usr/bin/env bats

setup_file() {
    load 'common-setup'
    _common_setup
}

@test "can inspect a DockerHub public image" {
    ./manifest-tool inspect alpine:latest
}

@test "can inspect a K8s public registry image" {
    ./manifest-tool inspect registry.k8s.io/pause:3.7
}

@test "can inspect a public ECR image on AWS" {
    ./manifest-tool inspect public.ecr.aws/datadog/agent:7
}