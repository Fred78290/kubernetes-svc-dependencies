ALL_ARCH = amd64 arm64

all: $(addprefix build-arch-,$(ALL_ARCH))

VERSION_MAJOR ?= 1
VERSION_MINOR ?= 21
VERSION_BUILD ?= 0
TAG?=v$(VERSION_MAJOR).$(VERSION_MINOR).$(VERSION_BUILD)
FLAGS=
ENVVAR=
GOOS?=$(shell go env GOOS)
GOARCH?=$(shell go env GOARCH)
REGISTRY?=fred78290
BUILD_DATE?=`date +%Y-%m-%dT%H:%M:%SZ`
VERSION_LDFLAGS=-X main.phVersion=$(TAG)

IMAGE=$(REGISTRY)/kubernetes-svc-dependencies

deps:
	go mod vendor

build: $(addprefix build-arch-,$(ALL_ARCH))

build-arch-%: deps clean-arch-%
	$(ENVVAR) GOOS=$(GOOS) GOARCH=$* go build -ldflags="-X main.phVersion=$(TAG) -X main.phBuildDate=$(BUILD_DATE)" -a -o out/$(GOOS)/$*/kubernetes-svc-dependencies

test-unit: clean build
	go test --test.short -race ./...

make-image: $(addprefix make-image-arch-,$(ALL_ARCH))

make-image-arch-%:
	docker build --pull -t ${IMAGE}:${TAG}-$* -f Dockerfile.$* .
	@echo "Image ${IMAGE}:${TAG}-$* completed"

push-image: $(addprefix push-image-arch-,$(ALL_ARCH))

push-image-arch-%:
	docker push ${IMAGE}:${TAG}-$*

push-manifest:
	docker buildx build --pull --platform linux/amd64,linux/arm64 --push -t ${IMAGE}:${TAG} .
	@echo "Image ${TAG}* completed"

clean: $(addprefix clean-arch-,$(ALL_ARCH))

clean-arch-%:
	rm -f ./out/$(GOOS)/$*/kubernetes-svc-dependencies

docker-builder:
	docker build -t kubernetes-svc-dependencies-builder ./builder

build-in-docker: $(addprefix build-in-docker-arch-,$(ALL_ARCH))

build-in-docker-arch-%: clean-arch-% docker-builder
	docker run --rm -v `pwd`:/gopath/src/github.com/Fred78290/kubernetes-svc-dependencies/ kubernetes-svc-dependencies-builder:latest bash \
		-c 'cd /gopath/src/github.com/Fred78290/kubernetes-svc-dependencies \
		&& make -e REGISTRY=${REGISTRY} -e TAG=${TAG} -e BUILD_DATE=`date +%Y-%m-%dT%H:%M:%SZ` build-arch-$*'

container: build-in-docker make-image

test-in-docker: clean docker-builder
	docker run -v `pwd`:/gopath/src/github.com/Fred78290/kubernetes-svc-dependencies/ kubernetes-svc-dependencies-builder:latest \
		bash -c 'cd /gopath/src/github.com/Fred78290/kubernetes-svc-dependencies && bash ./scripts/run-tests.sh'

.PHONY: all build test-unit clean format docker-builder build-in-docker release generate push-image push-manifest

