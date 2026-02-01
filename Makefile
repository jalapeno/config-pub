REGISTRY_NAME?=docker.io/iejalapeno
IMAGE_VERSION?=latest
.PHONY: all config-pub config-pub-container push clean test

ifdef V
TESTARGS = -v -args -alsologtostderr -v 5
else
TESTARGS =
endif

all: config-pub

config-pub:
	mkdir -p bin
	$(MAKE) -C ./cmd/config-pub compile-config-pub

config-pub-container: config-pub
	docker build -t $(REGISTRY_NAME)/config-pub:$(IMAGE_VERSION) -f ./Dockerfile .

push: config-pub-container
	docker push $(REGISTRY_NAME)/config-pub:$(IMAGE_VERSION)

clean:
	rm -rf bin

