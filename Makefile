all: build
build:
	CGO_ENABLED=0 GOOS=linux go build -o llm-gateway .
	docker build -t ultramangaia/gaiasec-env:llm-gateway . -f Dockerfile_local
push:
	docker push ultramangaia/gaiasec-env:llm-gateway
