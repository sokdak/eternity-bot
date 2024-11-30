VERSION ?= 0.0.1
IMAGE_TAG_BASE ?= docker.io/sokdak/eternity-bot
IMAGE ?= $(IMAGE_TAG_BASE):v$(VERSION)

.PHONY: build
build: fmt vet
	go build -o eternity-bot main.go

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: test
test:
	go test -v ./... -coverprofile coverage.out

.PHONY: docker-build
docker-build:
	docker build -t $(IMAGE) .

.PHONY: docker-push
docker-push:
	docker push $(IMAGE)

.PHONY: helm-build
helm-build:
	cd deploy/eternity-bot && helm dependency build
	helm package helm/eternity-bot --version $(VERSION) --app-version $(VERSION)

.PHONY: helm-push
helm-push:
	helm registry login
	helm push eternity-bot-$(VERSION).tgz eternity-bot
