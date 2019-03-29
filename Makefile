TAG?=latest

.PHONY: all
all: clean build

.PHONY: build
build: clean
	go build .
	mkdir bin
	mv satellite ./bin/

.PHONY: docker
docker: clean
	./build.sh

.PHONY: redist
redist: clean
	./build_redist.sh

.PHONY: ci-armhf-push
ci-armhf-push:
	(docker push s8sg/satellite:$(TAG)-armhf)

.PHONY: ci-armhf-build
ci-armhf-build:
	(./build.sh $(TAG)-armhf)

.PHONY: ci-arm64-push
ci-arm64-push:
	(docker push s8sg/satellite:$(TAG)-arm64)

.PHONY: ci-arm64-build
ci-arm64-build:
	(./build.sh $(TAG)-arm64)

.PHONY: clean
clean:
	(rm -rf ./bin)
