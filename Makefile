IMAGE_REPO ?= 172.31.30.52:5000/ultramangaia/gaiasec-env
IMAGE_TAG := llm-gateway

all: build
build:
	CGO_ENABLED=0 GOOS=linux go build -o llm-gateway .
	docker build -t $(IMAGE_REPO):$(IMAGE_TAG) . -f Dockerfile_local
push:
	docker push $(IMAGE_REPO):$(IMAGE_TAG)
