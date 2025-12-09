
BINARY=bin/api
GO_TEST=go test ./...
IMAGE_NAME ?= kidpech/api_free_demo:latest

.PHONY: run build test test-integration lint docker-build docker docker-run docker-push docker-logs docker-stop docker-up docker-down migrate seed demo-users

run:
	go run ./cmd/api

build:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY) ./cmd/api

test:
	$(GO_TEST)

lint:
	golangci-lint run ./...

test-integration:
	go test -tags=integration ./internal/tests/integration

migrate:
	migrate -path migrations/postgres -database "$$DB_DSN" up

seed:
	go run ./scripts/demo_seed.go --base-url=$${BASE_URL:-http://localhost:8080}

# Build docker image locally (uses Dockerfile at repo root)
docker-build:
	docker build -t $(IMAGE_NAME) -f Dockerfile .

# Alias: build image
docker: docker-build

# Run container locally (reads env from .env)
docker-run:
	docker run --rm -p 8080:8080 --env-file .env $(IMAGE_NAME)

# Push image to registry (requires docker login)
docker-push:
	docker push $(IMAGE_NAME)

# Follow logs for running container built from image
docker-logs:
	@docker ps --filter ancestor=$(IMAGE_NAME) --format "{{.ID}}" | xargs -r docker logs -f

# Stop any running container for this image
docker-stop:
	@docker ps --filter ancestor=$(IMAGE_NAME) --format "{{.ID}}" | xargs -r docker stop

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down -v
