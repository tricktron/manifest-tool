.PHONY: all binary build clean install install shell test-integration

PREFIX ?= ${DESTDIR}/usr
INSTALLDIR=${PREFIX}/bin
MANINSTALLDIR=${PREFIX}/share/man

GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null)
DOCKER_IMAGE := manifest-tool-dev$(if $(GIT_BRANCH),:$(GIT_BRANCH))
# set env like gobuildtag?
DOCKER_FLAGS := docker run --rm -i #$(DOCKER_ENVS)
# if this session isn't interactive, then we don't want to allocate a
# TTY, which would fail, but if it is interactive, we do want to attach
# so that the user can send e.g. ^C through.
INTERACTIVE := $(shell [ -t 0 ] && echo 1 || echo 0)
ifeq ($(INTERACTIVE), 1)
	DOCKER_FLAGS += -t
endif
DOCKER_RUN_DOCKER := $(DOCKER_FLAGS) "$(DOCKER_IMAGE)"

all: binary

binary:
	go build -o ${DEST}manifest github.com/estesp/manifest-tool

build-container:
	docker build ${DOCKER_BUILD_ARGS} -t "$(DOCKER_IMAGE)" .

clean:
	rm -f manifest

install:
	install -d -m 0755 ${INSTALLDIR}
	install -m 755 manifest ${INSTALLDIR}

shell: build-container
	$(DOCKER_RUN_DOCKER) bash

test-integration: build-container
	$(DOCKER_RUN_DOCKER) hack/make.sh test-integration

validate: build-container
	$(DOCKER_RUN_DOCKER) hack/make.sh validate-git-marks validate-gofmt validate-lint validate-vet
