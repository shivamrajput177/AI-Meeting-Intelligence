.PHONY: build lint test vet fmt up down logs

build:
	go build ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

lint:
	golangci-lint run ./...

test:
	go test ./...

# Phase 1 quickstart — see README.md for the curl flow to run after this.
up:
	docker compose up --build

down:
	docker compose down -v
