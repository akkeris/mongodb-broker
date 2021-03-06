# If the USE_SUDO_FOR_DOCKER env var is set, prefix docker commands with 'sudo'
ifdef USE_SUDO_FOR_DOCKER
	SUDO_CMD = sudo
endif

NAME ?= mongodb-broker
BASE_REPO ?= github.com/akkeris/$(NAME)
IMAGE ?= docker.io/akkeris/$(NAME)
TAG ?= $(shell git describe --tags --always)
PULL ?= IfNotPresent

build: ## Builds mongodb-broker
	go build -i  -o $(NAME) $(BASE_REPO)/cmd/servicebroker

test: ## Runs the tests
	go get github.com/smartystreets/goconvey
	go test -timeout 5s -v github.com/akkeris/mongodb-broker/pkg/broker -args -logtostderr=1 -stderrthreshold=4 -v 4

coverage: ## Runs the tests
	go get github.com/smartystreets/goconvey
	go test -timeout 5s -coverprofile cover.out -v github.com/akkeris/mongodb-broker/pkg/broker -args -logtostderr=1 -stderrthreshold=4 -v 4

run: image ## runs docker container local for testing
	docker run --name=mdb --env-file=.test.env -p 4242:4242 $(NAME) ./start.sh -v 4

image: ## Builds image local
	docker build -t $(NAME) .

#linux: ## Builds a Linux executable
#	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
#	go build -o servicebroker-linux --ldflags="-s" $(BASE_REPO)/cmd/servicebroker

#image: linux ## Builds a Linux based image
#	cp servicebroker-linux image/servicebroker
#	$(SUDO_CMD) docker build image/ -t "$(IMAGE):$(TAG)"

clean: ## Cleans up build artifacts
	rm -f $(NAME)
#	rm -f servicebroker-linux
#	rm -f image/servicebroker

push: image ## Pushes the image to dockerhub, REQUIRES SPECIAL PERMISSION
	$(SUDO_CMD) docker push "$(IMAGE):$(TAG)"

deploy-helm: image ## Deploys image with helm
	helm upgrade --install broker-skeleton --namespace broker-skeleton \
	charts/servicebroker \
	--set image="$(IMAGE):$(TAG)",imagePullPolicy="$(PULL)"

#deploy-openshift: image ## Deploys image to openshift
#	oc project osb-starter-pack || oc new-project osb-starter-pack
#	openshift/deploy.sh $(IMAGE):$(TAG)

create-ns: ## Cleans up the namespaces
	kubectl create ns test-ns

provision: create-ns ## Provisions a service instance
	kubectl apply -f manifests/service-instance.yaml

bind: ## Creates a binding
	kubectl apply -f manifests/service-binding.yaml

help: ## Shows the help
	@echo 'Usage: make <OPTIONS> ... <TARGETS>'
	@echo ''
	@echo 'Available targets are:'
	@echo ''
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
        awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ''

.PHONY: build test linux image clean push deploy-helm deploy-openshift create-ns provision bind help
