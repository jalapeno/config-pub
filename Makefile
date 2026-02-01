REGISTRY_NAME?=docker.io/iejalapeno
IMAGE_VERSION?=latest
.PHONY: all config-pub config-ingest config-pub-container config-ingest-container push clean test

ifdef V
TESTARGS = -v -args -alsologtostderr -v 5
else
TESTARGS =
endif

all: config-pub config-ingest

config-pub:
	mkdir -p bin
	$(MAKE) -C ./cmd/config-pub compile-config-pub

config-ingest:
	mkdir -p bin
	$(MAKE) -C ./cmd/config-ingest compile-config-ingest

config-pub-container: config-pub
	docker build -t $(REGISTRY_NAME)/config-pub:$(IMAGE_VERSION) -f ./Dockerfile.config-publish .

config-ingest-container: config-ingest
	docker build -t $(REGISTRY_NAME)/config-ingest:$(IMAGE_VERSION) -f ./Dockerfile.config-ingest .

push: config-pub-container config-ingest-container
	docker push $(REGISTRY_NAME)/config-pub:$(IMAGE_VERSION)
	docker push $(REGISTRY_NAME)/config-ingest:$(IMAGE_VERSION)

clean:
	rm -rf bin

