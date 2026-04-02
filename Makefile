REGISTRY ?= 172.31.30.52:5000
IMAGE_REPO ?= ultramangaia/gaiasec-env
IMAGE_TAG := llm-gateway
LOCAL_IMAGE := $(IMAGE_REPO):$(IMAGE_TAG)
REMOTE_IMAGE := $(REGISTRY)/$(IMAGE_REPO):$(IMAGE_TAG)

all:
	@echo "Targets: "
	@make -qpRr | egrep -e '^[a-z].*:$$' | sed -e 's~:~~g' | grep -v 'all' | sort
pull:
	git checkout master
	git pull
commit:
	test -z "$$(git status --short)" || opencode run 'commit it'
build:
	CGO_ENABLED=0 GOOS=linux go build -o llm-gateway .
	docker build -t $(LOCAL_IMAGE) . -f Dockerfile_local
push:
	test -z "$$(git cherry -v)" || opencode run 'push it'

push_image:
	docker tag $(LOCAL_IMAGE) $(REMOTE_IMAGE)
	docker push $(REMOTE_IMAGE)
push_image_remote:
	docker push $(LOCAL_IMAGE)
